package accounts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	cleerrors "github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/errors"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

const (
	microsoftTimeout     = 30 * time.Second
	microsoftMaxResponse = 4 << 20
	microsoftScope       = "XboxLive.signin offline_access"
)

// microsoftClientIDEnv is the environment variable used to supply the OAuth
// public-client application ID when neither programmatic options nor the
// configuration file provide one.
const microsoftClientIDEnv = "CLE_MICROSOFT_CLIENT_ID"

var (
	microsoftProfileIDPattern   = regexp.MustCompile(`(?i)^[0-9a-f]{32}$`)
	microsoftProfileNamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,16}$`)
	microsoftXUIDPattern        = regexp.MustCompile(`^[0-9A-Fa-f]{16}$`)
)

// MicrosoftEndpoints allows tests and advanced integrations to override the
// official Microsoft / Xbox / Minecraft service endpoints. Zero values resolve
// to the official endpoints.
type MicrosoftEndpoints struct {
	OAuthDeviceCode       string
	OAuthToken            string
	XboxLive              string
	XSTS                  string
	MinecraftLogin        string
	MinecraftEntitlements string
	MinecraftProfile      string
}

// MicrosoftOptions configures the Microsoft account provider.
type MicrosoftOptions struct {
	ClientID    string
	Credentials CredentialStore
	HTTPClient  *http.Client
	Endpoints   MicrosoftEndpoints
}

// microsoftSession is stored in the operating-system credential store. The OAuth
// refresh token, OAuth access token, and Minecraft access token are kept in
// distinct fields and are never written to account JSON or logs.
type microsoftSession struct {
	AccessToken          string    `json:"accessToken"`
	RefreshToken         string    `json:"refreshToken"`
	ExpiresAt            time.Time `json:"expiresAt,omitempty"`
	MinecraftAccessToken string    `json:"minecraftAccessToken"`
	MinecraftExpiresAt   time.Time `json:"minecraftExpiresAt,omitempty"`
	XUID                 string    `json:"xuid,omitempty"`
}

type MicrosoftProvider struct {
	credentials CredentialStore
	client      *http.Client
	clientID    string
	endpoints   MicrosoftEndpoints
	locksMu     sync.Mutex
	locks       map[string]*accountLock
}

// DeviceCodeAuthenticator is implemented by providers that use the OAuth 2.0
// device authorization grant and must surface the verification URL and user
// code before polling begins.
type DeviceCodeAuthenticator interface {
	StartDeviceCode(context.Context) (models.DeviceCode, error)
	AuthenticateDeviceCode(context.Context, models.DeviceCode) (models.Account, error)
}

func NewMicrosoftProvider(options MicrosoftOptions) (*MicrosoftProvider, error) {
	if options.Credentials == nil {
		return nil, errors.New("Microsoft provider requires an operating-system credential store")
	}
	clientID := strings.TrimSpace(options.ClientID)
	if clientID == "" {
		clientID = strings.TrimSpace(os.Getenv(microsoftClientIDEnv))
	}
	client := &http.Client{Timeout: microsoftTimeout}
	if options.HTTPClient != nil {
		copy := *options.HTTPClient
		if copy.Timeout <= 0 {
			copy.Timeout = microsoftTimeout
		}
		client = &copy
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	endpoints := resolveMicrosoftEndpoints(options.Endpoints)
	return &MicrosoftProvider{
		credentials: options.Credentials,
		client:      client,
		clientID:    clientID,
		endpoints:   endpoints,
		locks:       make(map[string]*accountLock),
	}, nil
}

func resolveMicrosoftEndpoints(overrides MicrosoftEndpoints) MicrosoftEndpoints {
	official := MicrosoftEndpoints{
		OAuthDeviceCode:       "https://login.microsoftonline.com/consumers/oauth2/v2.0/devicecode",
		OAuthToken:            "https://login.microsoftonline.com/consumers/oauth2/v2.0/token",
		XboxLive:              "https://user.auth.xboxlive.com/user/authenticate",
		XSTS:                  "https://xsts.auth.xboxlive.com/xsts/authorize",
		MinecraftLogin:        "https://api.minecraftservices.com/authentication/login_with_xbox",
		MinecraftEntitlements: "https://api.minecraftservices.com/entitlements/mcstore",
		MinecraftProfile:      "https://api.minecraftservices.com/minecraft/profile",
	}
	if overrides.OAuthDeviceCode != "" {
		official.OAuthDeviceCode = overrides.OAuthDeviceCode
	}
	if overrides.OAuthToken != "" {
		official.OAuthToken = overrides.OAuthToken
	}
	if overrides.XboxLive != "" {
		official.XboxLive = overrides.XboxLive
	}
	if overrides.XSTS != "" {
		official.XSTS = overrides.XSTS
	}
	if overrides.MinecraftLogin != "" {
		official.MinecraftLogin = overrides.MinecraftLogin
	}
	if overrides.MinecraftEntitlements != "" {
		official.MinecraftEntitlements = overrides.MinecraftEntitlements
	}
	if overrides.MinecraftProfile != "" {
		official.MinecraftProfile = overrides.MinecraftProfile
	}
	return official
}

func (provider *MicrosoftProvider) ClientID() string {
	return provider.clientID
}

func (*MicrosoftProvider) Type() models.AccountType {
	return models.AccountMicrosoft
}

// Authenticate is part of the generic AccountProvider interface. Microsoft
// authentication is interactive and must be driven through the two-phase
// DeviceCodeAuthenticator API so the verification URL and user code can be
// shown to the user before polling.
func (provider *MicrosoftProvider) Authenticate(context.Context, models.AccountAuthRequest) (models.Account, error) {
	return models.Account{}, cleerrors.New(cleerrors.CodeAuthenticationFailed,
		"Microsoft account authentication uses the OAuth device code flow; use StartDeviceCode and AuthenticateDeviceCode")
}

func (provider *MicrosoftProvider) requireClientID() error {
	if strings.TrimSpace(provider.clientID) == "" {
		return cleerrors.New(cleerrors.CodeInvalidConfiguration,
			"a Microsoft OAuth public-client application ID is required; set account.microsoft.clientId in the configuration file or the "+microsoftClientIDEnv+" environment variable")
	}
	return nil
}

func (provider *MicrosoftProvider) lockAccount(accountID string) func() {
	provider.locksMu.Lock()
	lock := provider.locks[accountID]
	if lock == nil {
		lock = &accountLock{}
		provider.locks[accountID] = lock
	}
	lock.refs++
	provider.locksMu.Unlock()
	lock.mu.Lock()
	return func() {
		lock.mu.Unlock()
		provider.locksMu.Lock()
		lock.refs--
		if lock.refs == 0 {
			delete(provider.locks, accountID)
		}
		provider.locksMu.Unlock()
	}
}

// Refresh re-runs the OAuth token refresh and the full Xbox/XSTS/Minecraft
// chain to obtain a fresh Minecraft access token and game profile.
func (provider *MicrosoftProvider) Refresh(ctx context.Context, account models.Account) (models.Account, error) {
	unlock := provider.lockAccount(account.ID)
	defer unlock()
	if err := validateMicrosoftAccount(account); err != nil {
		return models.Account{}, err
	}
	session, err := provider.loadSession(ctx, account)
	if err != nil {
		return models.Account{}, err
	}
	oauth, err := provider.refreshOAuth(ctx, session)
	if err != nil {
		return models.Account{}, err
	}
	refreshed, err := provider.completeAuthentication(ctx, account, oauth)
	if err != nil {
		return models.Account{}, err
	}
	refreshed.Session = models.AccountSession{}
	return refreshed, nil
}

// Validate performs a local structural check of the account and its stored
// credentials. It does not make network calls.
func (provider *MicrosoftProvider) Validate(ctx context.Context, account models.Account) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateMicrosoftAccount(account); err != nil {
		return err
	}
	session, err := provider.loadSession(ctx, account)
	if err != nil {
		return err
	}
	if strings.TrimSpace(session.RefreshToken) == "" {
		return cleerrors.New(cleerrors.CodeSessionInvalid, "Microsoft account has no refresh token")
	}
	return nil
}

func (provider *MicrosoftProvider) GetProfile(ctx context.Context, account models.Account) (models.GameProfile, error) {
	if err := ctx.Err(); err != nil {
		return models.GameProfile{}, err
	}
	if err := validateMicrosoftAccount(account); err != nil {
		return models.GameProfile{}, err
	}
	return cloneProfile(account.Profile), nil
}

func (provider *MicrosoftProvider) Logout(ctx context.Context, account models.Account) error {
	unlock := provider.lockAccount(account.ID)
	defer unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := provider.credentials.Delete(context.WithoutCancel(ctx), account.CredentialRef); err != nil {
		return cleerrors.New(cleerrors.CodeAuthenticationFailed, "could not delete Microsoft credentials from the operating-system credential store")
	}
	return nil
}

func (provider *MicrosoftProvider) DeleteAccountCredentials(ctx context.Context, account models.Account) error {
	if account.Type != models.AccountMicrosoft {
		return nil
	}
	unlock := provider.lockAccount(account.ID)
	defer unlock()
	if account.ID == "" || account.CredentialRef != microsoftCredentialKey(account.ID) {
		return cleerrors.New(cleerrors.CodeAuthenticationFailed, "Microsoft credential reference is invalid")
	}
	if err := provider.credentials.Delete(context.WithoutCancel(ctx), account.CredentialRef); err != nil {
		return cleerrors.New(cleerrors.CodeAuthenticationFailed, "could not delete Microsoft credentials from the operating-system credential store")
	}
	return nil
}

// ResolveSession returns the current launch session for a Microsoft account.
func (provider *MicrosoftProvider) ResolveSession(ctx context.Context, account models.Account) (models.AccountSession, error) {
	unlock := provider.lockAccount(account.ID)
	defer unlock()
	resolved, err := provider.resolveLaunchAccount(ctx, account)
	if err != nil {
		return models.AccountSession{}, err
	}
	return resolved.Session, nil
}

// ResolveLaunchAccount ensures the account has a usable Minecraft access token,
// refreshing the underlying OAuth token and re-running the Xbox/XSTS/Minecraft
// chain only when the cached Minecraft token is missing or expired.
func (provider *MicrosoftProvider) ResolveLaunchAccount(ctx context.Context, account models.Account) (models.Account, error) {
	unlock := provider.lockAccount(account.ID)
	defer unlock()
	return provider.resolveLaunchAccount(ctx, account)
}

func (provider *MicrosoftProvider) resolveLaunchAccount(ctx context.Context, account models.Account) (models.Account, error) {
	if err := validateMicrosoftAccount(account); err != nil {
		return models.Account{}, err
	}
	session, err := provider.loadSession(ctx, account)
	if err != nil {
		return models.Account{}, err
	}
	if session.MinecraftAccessToken != "" && time.Now().Before(session.MinecraftExpiresAt) {
		account.Session = models.AccountSession{
			AccessToken: session.MinecraftAccessToken,
			ExpiresAt:   session.MinecraftExpiresAt,
		}
		account.XUID = session.XUID
		return account, nil
	}
	oauth, err := provider.ensureOAuthToken(ctx, session)
	if err != nil {
		return models.Account{}, err
	}
	resolved, err := provider.completeAuthentication(ctx, account, oauth)
	if err != nil {
		return models.Account{}, err
	}
	resolved.Session = models.AccountSession{
		AccessToken: resolved.Session.AccessToken,
		ExpiresAt:   resolved.Session.ExpiresAt,
	}
	return resolved, nil
}

func (provider *MicrosoftProvider) loadSession(ctx context.Context, account models.Account) (microsoftSession, error) {
	if err := validateMicrosoftAccount(account); err != nil {
		return microsoftSession{}, err
	}
	data, err := provider.credentials.Retrieve(ctx, account.CredentialRef)
	if err != nil {
		return microsoftSession{}, cleerrors.New(cleerrors.CodeAuthenticationFailed, "Microsoft credentials are unavailable in the operating-system credential store")
	}
	defer zeroBytes(data)
	var session microsoftSession
	if err := json.Unmarshal(data, &session); err != nil {
		return microsoftSession{}, cleerrors.New(cleerrors.CodeAuthenticationFailed, "stored Microsoft credentials are invalid")
	}
	return session, nil
}

func (provider *MicrosoftProvider) storeSession(ctx context.Context, key string, session microsoftSession) error {
	data, err := json.Marshal(session)
	if err != nil {
		return cleerrors.Wrap(cleerrors.CodeAuthenticationFailed, "encode Microsoft credentials", err)
	}
	defer zeroBytes(data)
	if err := provider.credentials.Store(ctx, key, data); err != nil {
		return cleerrors.New(cleerrors.CodeAuthenticationFailed, "could not store Microsoft credentials securely in the operating-system credential store")
	}
	return nil
}

func microsoftCredentialKey(accountID string) string {
	return "microsoft:" + accountID
}

func validateMicrosoftAccount(account models.Account) error {
	if account.Type != models.AccountMicrosoft || account.ID == "" ||
		account.CredentialRef != microsoftCredentialKey(account.ID) ||
		!microsoftProfileIDPattern.MatchString(account.Profile.ID) ||
		!microsoftProfileNamePattern.MatchString(account.Profile.Name) ||
		account.Name != account.Profile.Name ||
		strings.ToLower(account.UUID) != strings.ToLower(account.Profile.ID) {
		return cleerrors.New(cleerrors.CodeAuthenticationFailed, "Microsoft account profile is invalid")
	}
	return nil
}

// doJSON sends a JSON request and returns the status code and body. The body is
// bounded and TLS verification is never disabled.
func (provider *MicrosoftProvider) doJSON(ctx context.Context, method, rawURL string, payload any, bearer string) (int, []byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, cleerrors.Wrap(cleerrors.CodeAuthenticationFailed, "encode Microsoft authentication request", err)
	}
	defer zeroBytes(data)
	return provider.do(ctx, method, rawURL, data, "application/json", bearer)
}

// doForm sends an application/x-www-form-urlencoded request.
func (provider *MicrosoftProvider) doForm(ctx context.Context, rawURL string, form url.Values, bearer string) (int, []byte, error) {
	data := []byte(form.Encode())
	defer zeroBytes(data)
	return provider.do(ctx, http.MethodPost, rawURL, data, "application/x-www-form-urlencoded", bearer)
}

func (provider *MicrosoftProvider) do(ctx context.Context, method, rawURL string, body []byte, contentType, bearer string) (int, []byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewReader(body))
	if err != nil {
		return 0, nil, cleerrors.Wrap(cleerrors.CodeInvalidConfiguration, "create Microsoft authentication request", err)
	}
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Accept", "application/json")
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	response, err := provider.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return 0, nil, ctx.Err()
		}
		return 0, nil, cleerrors.New(cleerrors.CodeAuthenticationFailed, "Microsoft authentication request failed; check the network connection")
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, microsoftMaxResponse+1))
	if err != nil {
		if ctx.Err() != nil {
			return 0, nil, ctx.Err()
		}
		return 0, nil, cleerrors.New(cleerrors.CodeAuthenticationFailed, "could not read the Microsoft authentication response")
	}
	if len(responseBody) > microsoftMaxResponse {
		return 0, nil, cleerrors.New(cleerrors.CodeAuthenticationFailed, "Microsoft authentication response exceeds the size limit")
	}
	return response.StatusCode, responseBody, nil
}

func decodeMicrosoftJSON(data []byte, output any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(output); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func microsoftStatusError(operation string, status int) error {
	message := fmt.Sprintf("%s returned HTTP %d", operation, status)
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return cleerrors.New(cleerrors.CodeAuthenticationFailed, message)
	case status >= 400 && status < 500:
		return cleerrors.New(cleerrors.CodeAuthenticationFailed, message)
	default:
		return cleerrors.New(cleerrors.CodeAuthenticationFailed, message)
	}
}
