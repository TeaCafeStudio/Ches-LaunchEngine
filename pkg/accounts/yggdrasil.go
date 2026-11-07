package accounts

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/core"
	cleerrors "github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/errors"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

const (
	yggdrasilAgentVersion     = 1
	yggdrasilTimeout          = 15 * time.Second
	yggdrasilMaxResponse      = 1 << 20
	yggdrasilMaxProperties    = 128
	yggdrasilMaxPropertyBytes = 256 << 10
)

var yggdrasilProfileIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{32}$`)
var yggdrasilProfileNamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,16}$`)
var yggdrasilPropertyNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

type YggdrasilProvider struct {
	credentials CredentialStore
	client      *http.Client
	allowHTTP   bool
	locksMu     sync.Mutex
	locks       map[string]*accountLock
}

type accountLock struct {
	mu   sync.Mutex
	refs int
}

type YggdrasilOptions struct {
	Credentials CredentialStore
	HTTPClient  *http.Client
	AllowHTTP   bool
}

type yggdrasilSession struct {
	AuthServer  string    `json:"authServer"`
	Username    string    `json:"username"`
	AccessToken string    `json:"accessToken"`
	ClientToken string    `json:"clientToken"`
	ExpiresAt   time.Time `json:"expiresAt,omitempty"`
}

type yggdrasilProfile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type yggdrasilUserProperty struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Signature string `json:"signature,omitempty"`
}

type yggdrasilUser struct {
	ID         string                  `json:"id"`
	Properties []yggdrasilUserProperty `json:"properties"`
}

type yggdrasilResponse struct {
	AccessToken       string             `json:"accessToken"`
	ClientToken       string             `json:"clientToken"`
	AvailableProfiles []yggdrasilProfile `json:"availableProfiles"`
	SelectedProfile   *yggdrasilProfile  `json:"selectedProfile"`
	User              *yggdrasilUser     `json:"user"`
	Error             string             `json:"error"`
	ErrorMessage      string             `json:"errorMessage"`
}

type yggdrasilAgent struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
}

type yggdrasilAuthenticateRequest struct {
	Agent       yggdrasilAgent `json:"agent"`
	Username    string         `json:"username"`
	Password    string         `json:"password"`
	ClientToken string         `json:"clientToken"`
	RequestUser bool           `json:"requestUser"`
}

type yggdrasilTokenRequest struct {
	AccessToken     string            `json:"accessToken"`
	ClientToken     string            `json:"clientToken"`
	SelectedProfile *yggdrasilProfile `json:"selectedProfile,omitempty"`
	RequestUser     bool              `json:"requestUser,omitempty"`
}

type yggdrasilSignoutRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func NewYggdrasilProvider(options YggdrasilOptions) (*YggdrasilProvider, error) {
	if options.Credentials == nil {
		return nil, errors.New("Yggdrasil provider requires an operating-system credential store")
	}
	client := &http.Client{Timeout: yggdrasilTimeout}
	if options.HTTPClient != nil {
		copy := *options.HTTPClient
		if copy.Timeout <= 0 {
			copy.Timeout = yggdrasilTimeout
		}
		client = &copy
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &YggdrasilProvider{
		credentials: options.Credentials,
		client:      client,
		allowHTTP:   options.AllowHTTP,
		locks:       make(map[string]*accountLock),
	}, nil
}

func (*YggdrasilProvider) Type() models.AccountType {
	return models.AccountYggdrasil
}

func (provider *YggdrasilProvider) Authenticate(ctx context.Context, request models.AccountAuthRequest) (models.Account, error) {
	if err := ctx.Err(); err != nil {
		return models.Account{}, err
	}
	if request.Type != models.AccountYggdrasil {
		return models.Account{}, fmt.Errorf("%w: Yggdrasil provider cannot authenticate %q accounts", core.ErrAccountOperationUnsupported, request.Type)
	}
	if strings.TrimSpace(request.Username) == "" || request.Password == "" {
		return models.Account{}, cleerrors.New(cleerrors.CodeAuthenticationFailed, "Yggdrasil username and password are required")
	}
	baseURL, err := validateYggdrasilServer(request.AuthServer, provider.allowHTTP)
	if err != nil {
		return models.Account{}, cleerrors.Wrap(cleerrors.CodeInvalidConfiguration, "invalid Yggdrasil authentication server URL", err)
	}
	clientToken, err := newClientToken()
	if err != nil {
		return models.Account{}, fmt.Errorf("generate Yggdrasil client token")
	}
	payload := yggdrasilAuthenticateRequest{
		Agent:    yggdrasilAgent{Name: "Minecraft", Version: yggdrasilAgentVersion},
		Username: request.Username, Password: request.Password,
		ClientToken: clientToken, RequestUser: true,
	}
	response, err := provider.post(ctx, baseURL, "authenticate", payload)
	if err != nil {
		return models.Account{}, err
	}
	if response.ClientToken != clientToken {
		return models.Account{}, cleerrors.New(cleerrors.CodeAuthenticationFailed, "Yggdrasil server returned an invalid client token")
	}
	profile, err := selectedYggdrasilProfile(response)
	if err != nil {
		return models.Account{}, err
	}
	properties, _, err := yggdrasilUserProperties(response.User)
	if err != nil {
		return models.Account{}, err
	}
	accountID, err := newAccountID()
	if err != nil {
		return models.Account{}, fmt.Errorf("generate account ID")
	}
	now := time.Now().UTC()
	account := models.Account{
		ID: accountID, Name: profile.Name, UUID: profile.ID, DisplayName: profile.Name,
		Profile: models.GameProfile{ID: profile.ID, Name: profile.Name, Properties: properties},
		Type:    models.AccountYggdrasil, AuthServer: baseURL,
		CredentialRef: credentialKey(accountID), UserType: "legacy",
		CreatedAt: now, UpdatedAt: now,
	}
	session := yggdrasilSession{
		AuthServer: baseURL, Username: request.Username,
		AccessToken: response.AccessToken, ClientToken: response.ClientToken,
	}
	if err := provider.storeSession(ctx, account.CredentialRef, session); err != nil {
		return models.Account{}, err
	}
	return account, nil
}

func (provider *YggdrasilProvider) Refresh(ctx context.Context, account models.Account) (models.Account, error) {
	unlock := provider.lockAccount(account.ID)
	defer unlock()
	return provider.refresh(ctx, account)
}

func (provider *YggdrasilProvider) refresh(ctx context.Context, account models.Account) (models.Account, error) {
	session, err := provider.loadSession(ctx, account)
	if err != nil {
		return models.Account{}, err
	}
	payload := yggdrasilTokenRequest{
		AccessToken: session.AccessToken, ClientToken: session.ClientToken,
		SelectedProfile: &yggdrasilProfile{ID: account.Profile.ID, Name: account.Profile.Name},
		RequestUser:     true,
	}
	response, err := provider.post(ctx, session.AuthServer, "refresh", payload)
	if err != nil {
		return models.Account{}, err
	}
	if response.ClientToken != session.ClientToken {
		return models.Account{}, cleerrors.New(cleerrors.CodeAuthenticationFailed, "Yggdrasil server returned an invalid client token")
	}
	profile, err := selectedYggdrasilProfile(response)
	if err != nil {
		return models.Account{}, err
	}
	if profile.ID != account.Profile.ID {
		return models.Account{}, cleerrors.New(cleerrors.CodeAuthenticationFailed, "Yggdrasil refresh selected a different game profile")
	}
	session.AccessToken = response.AccessToken
	if properties, present, err := yggdrasilUserProperties(response.User); err != nil {
		return models.Account{}, err
	} else if present {
		account.Profile.Properties = properties
	}
	if err := provider.storeSession(ctx, account.CredentialRef, session); err != nil {
		return models.Account{}, err
	}
	account.Name, account.UUID, account.DisplayName = profile.Name, profile.ID, profile.Name
	account.Profile.ID, account.Profile.Name = profile.ID, profile.Name
	account.Session = models.AccountSession{}
	return account, nil
}

func (provider *YggdrasilProvider) Validate(ctx context.Context, account models.Account) error {
	unlock := provider.lockAccount(account.ID)
	defer unlock()
	return provider.validate(ctx, account)
}

func (provider *YggdrasilProvider) validate(ctx context.Context, account models.Account) error {
	session, err := provider.loadSession(ctx, account)
	if err != nil {
		return err
	}
	status, body, err := provider.do(ctx, session.AuthServer, "validate", yggdrasilTokenRequest{
		AccessToken: session.AccessToken, ClientToken: session.ClientToken,
	}, true)
	defer zeroBytes(body)
	if err != nil {
		return err
	}
	if status != http.StatusNoContent {
		return yggdrasilStatusError(status, nil)
	}
	return nil
}

func (provider *YggdrasilProvider) GetProfile(ctx context.Context, account models.Account) (models.GameProfile, error) {
	if err := ctx.Err(); err != nil {
		return models.GameProfile{}, err
	}
	if err := validateYggdrasilAccount(account); err != nil {
		return models.GameProfile{}, err
	}
	return cloneProfile(account.Profile), nil
}

func (provider *YggdrasilProvider) Logout(ctx context.Context, account models.Account) error {
	return provider.Invalidate(ctx, account)
}

func (provider *YggdrasilProvider) Invalidate(ctx context.Context, account models.Account) error {
	unlock := provider.lockAccount(account.ID)
	defer unlock()
	session, err := provider.loadSession(ctx, account)
	if err != nil {
		return err
	}
	status, body, err := provider.do(ctx, session.AuthServer, "invalidate", yggdrasilTokenRequest{
		AccessToken: session.AccessToken, ClientToken: session.ClientToken,
	}, true)
	defer zeroBytes(body)
	if err != nil {
		return err
	}
	if status != http.StatusNoContent {
		return yggdrasilStatusError(status, nil)
	}
	if err := provider.credentials.Delete(context.WithoutCancel(ctx), account.CredentialRef); err != nil {
		return cleerrors.New(cleerrors.CodeAuthenticationFailed, "could not delete Yggdrasil credentials from the operating-system credential store")
	}
	return nil
}

func (provider *YggdrasilProvider) Signout(ctx context.Context, account models.Account, password string) error {
	unlock := provider.lockAccount(account.ID)
	defer unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateYggdrasilAccount(account); err != nil {
		return err
	}
	if password == "" {
		return cleerrors.New(cleerrors.CodeAuthenticationFailed, "Yggdrasil password is required for signout")
	}
	session, err := provider.loadSession(ctx, account)
	if err != nil {
		return err
	}
	if strings.TrimSpace(session.Username) == "" {
		return cleerrors.New(cleerrors.CodeAuthenticationFailed, "stored Yggdrasil login username is unavailable")
	}
	status, body, err := provider.do(ctx, account.AuthServer, "signout", yggdrasilSignoutRequest{
		Username: session.Username, Password: password,
	}, true)
	defer zeroBytes(body)
	if err != nil {
		return err
	}
	if status != http.StatusNoContent {
		return yggdrasilStatusError(status, nil)
	}
	if err := provider.credentials.Delete(context.WithoutCancel(ctx), account.CredentialRef); err != nil {
		return cleerrors.New(cleerrors.CodeAuthenticationFailed, "could not delete Yggdrasil credentials from the operating-system credential store")
	}
	return nil
}

func (provider *YggdrasilProvider) DeleteAccountCredentials(ctx context.Context, account models.Account) error {
	if account.Type != models.AccountYggdrasil {
		return nil
	}
	unlock := provider.lockAccount(account.ID)
	defer unlock()
	if account.ID == "" || account.CredentialRef != credentialKey(account.ID) {
		return cleerrors.New(cleerrors.CodeAuthenticationFailed, "Yggdrasil credential reference is invalid")
	}
	if err := provider.credentials.Delete(context.WithoutCancel(ctx), account.CredentialRef); err != nil {
		return cleerrors.New(cleerrors.CodeAuthenticationFailed, "could not delete Yggdrasil credentials from the operating-system credential store")
	}
	return nil
}

func (provider *YggdrasilProvider) ResolveSession(ctx context.Context, account models.Account) (models.AccountSession, error) {
	unlock := provider.lockAccount(account.ID)
	defer unlock()
	resolved, err := provider.resolveLaunchAccount(ctx, account)
	if err != nil {
		return models.AccountSession{}, err
	}
	return resolved.Session, nil
}

func (provider *YggdrasilProvider) ResolveLaunchAccount(ctx context.Context, account models.Account) (models.Account, error) {
	unlock := provider.lockAccount(account.ID)
	defer unlock()
	return provider.resolveLaunchAccount(ctx, account)
}

func (provider *YggdrasilProvider) resolveLaunchAccount(ctx context.Context, account models.Account) (models.Account, error) {
	session, err := provider.loadSession(ctx, account)
	if err != nil {
		return models.Account{}, cleerrors.Wrap(cleerrors.CodeSessionInvalid, "stored Yggdrasil session is unavailable", err)
	}
	status, body, err := provider.do(ctx, session.AuthServer, "validate", yggdrasilTokenRequest{
		AccessToken: session.AccessToken, ClientToken: session.ClientToken,
	}, true)
	defer zeroBytes(body)
	if err != nil {
		return models.Account{}, cleerrors.Wrap(cleerrors.CodeSessionInvalid, "could not validate the Yggdrasil session", err)
	}
	if status == http.StatusNoContent {
		account.Session = launchSession(session)
		return account, nil
	}
	if status != http.StatusForbidden {
		return models.Account{}, cleerrors.Wrap(cleerrors.CodeSessionInvalid, "Yggdrasil session validation was rejected", yggdrasilStatusError(status, body))
	}
	refreshed, err := provider.refresh(ctx, account)
	if err != nil {
		return models.Account{}, cleerrors.Wrap(cleerrors.CodeSessionRefreshFailed, "Yggdrasil session was rejected and refresh failed", err)
	}
	session, err = provider.loadSession(ctx, refreshed)
	if err != nil {
		return models.Account{}, cleerrors.Wrap(cleerrors.CodeSessionRefreshFailed, "could not load the refreshed Yggdrasil session", err)
	}
	status, body, err = provider.do(ctx, session.AuthServer, "validate", yggdrasilTokenRequest{
		AccessToken: session.AccessToken, ClientToken: session.ClientToken,
	}, true)
	defer zeroBytes(body)
	if err != nil {
		return models.Account{}, cleerrors.Wrap(cleerrors.CodeSessionRefreshFailed, "could not validate the refreshed Yggdrasil session", err)
	}
	if status != http.StatusNoContent {
		return models.Account{}, cleerrors.Wrap(cleerrors.CodeSessionRefreshFailed, "refreshed Yggdrasil session was rejected", yggdrasilStatusError(status, nil))
	}
	refreshed.Session = launchSession(session)
	return refreshed, nil
}

func (provider *YggdrasilProvider) lockAccount(accountID string) func() {
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

func launchSession(session yggdrasilSession) models.AccountSession {
	return models.AccountSession{
		AccessToken: session.AccessToken, ClientToken: session.ClientToken,
		ExpiresAt: session.ExpiresAt,
	}
}

func (provider *YggdrasilProvider) loadSession(ctx context.Context, account models.Account) (yggdrasilSession, error) {
	if err := validateYggdrasilAccount(account); err != nil {
		return yggdrasilSession{}, err
	}
	data, err := provider.credentials.Retrieve(ctx, account.CredentialRef)
	if err != nil {
		return yggdrasilSession{}, cleerrors.New(cleerrors.CodeAuthenticationFailed, "Yggdrasil credentials are unavailable in the operating-system credential store")
	}
	defer zeroBytes(data)
	var session yggdrasilSession
	if err := json.Unmarshal(data, &session); err != nil ||
		strings.TrimSpace(session.AccessToken) == "" || strings.TrimSpace(session.ClientToken) == "" {
		return yggdrasilSession{}, cleerrors.New(cleerrors.CodeAuthenticationFailed, "stored Yggdrasil credentials are invalid")
	}
	if session.AuthServer != account.AuthServer {
		return yggdrasilSession{}, cleerrors.New(cleerrors.CodeAuthenticationFailed, "stored Yggdrasil server does not match the account")
	}
	return session, nil
}

func (provider *YggdrasilProvider) storeSession(ctx context.Context, key string, session yggdrasilSession) error {
	data, err := json.Marshal(session)
	if err != nil {
		return cleerrors.Wrap(cleerrors.CodeAuthenticationFailed, "encode Yggdrasil credentials", err)
	}
	defer zeroBytes(data)
	if err := provider.credentials.Store(ctx, key, data); err != nil {
		return cleerrors.New(cleerrors.CodeAuthenticationFailed, "could not store Yggdrasil credentials securely in the operating-system credential store")
	}
	return nil
}

func (provider *YggdrasilProvider) post(ctx context.Context, baseURL, endpoint string, payload any) (yggdrasilResponse, error) {
	status, body, err := provider.do(ctx, baseURL, endpoint, payload, false)
	if err != nil {
		return yggdrasilResponse{}, err
	}
	defer zeroBytes(body)
	if status != http.StatusOK {
		return yggdrasilResponse{}, yggdrasilStatusError(status, body)
	}
	var response yggdrasilResponse
	if err := decodeYggdrasilJSON(body, &response); err != nil {
		return yggdrasilResponse{}, cleerrors.Wrap(cleerrors.CodeAuthenticationFailed, "Yggdrasil server returned an invalid response", err)
	}
	if response.Error != "" {
		return yggdrasilResponse{}, yggdrasilServerError(response.Error)
	}
	if strings.TrimSpace(response.AccessToken) == "" {
		return yggdrasilResponse{}, cleerrors.New(cleerrors.CodeAuthenticationFailed, "Yggdrasil server returned an empty access token")
	}
	return response, nil
}

func (provider *YggdrasilProvider) do(ctx context.Context, baseURL, endpoint string, payload any, allowEmpty bool) (int, []byte, error) {
	target, err := endpointURL(baseURL, endpoint, provider.allowHTTP)
	if err != nil {
		return 0, nil, cleerrors.Wrap(cleerrors.CodeInvalidConfiguration, "invalid Yggdrasil authentication server URL", err)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, cleerrors.Wrap(cleerrors.CodeAuthenticationFailed, "encode Yggdrasil request", err)
	}
	defer zeroBytes(data)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(data))
	if err != nil {
		return 0, nil, cleerrors.Wrap(cleerrors.CodeInvalidConfiguration, "create Yggdrasil request", err)
	}
	defer zeroBytes(data)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := provider.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return 0, nil, ctx.Err()
		}
		return 0, nil, cleerrors.New(cleerrors.CodeAuthenticationFailed, "Yggdrasil request failed; check the authentication server address and network connection")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, yggdrasilMaxResponse+1))
	if err != nil {
		if ctx.Err() != nil {
			return 0, nil, ctx.Err()
		}
		return 0, nil, cleerrors.New(cleerrors.CodeAuthenticationFailed, "could not read the Yggdrasil server response")
	}
	if len(body) > yggdrasilMaxResponse {
		return 0, nil, cleerrors.New(cleerrors.CodeAuthenticationFailed, "Yggdrasil response exceeds the size limit")
	}
	if !allowEmpty && response.StatusCode == http.StatusOK && len(bytes.TrimSpace(body)) == 0 {
		return 0, nil, cleerrors.New(cleerrors.CodeAuthenticationFailed, "Yggdrasil server returned an empty response")
	}
	return response.StatusCode, body, nil
}

func selectedYggdrasilProfile(response yggdrasilResponse) (yggdrasilProfile, error) {
	if response.SelectedProfile == nil {
		return yggdrasilProfile{}, cleerrors.New(cleerrors.CodeProfileNotFound, "Yggdrasil server did not select a game profile")
	}
	profile := *response.SelectedProfile
	profile.ID = normalizeProfileID(profile.ID)
	if !yggdrasilProfileIDPattern.MatchString(profile.ID) || !yggdrasilProfileNamePattern.MatchString(profile.Name) {
		return yggdrasilProfile{}, cleerrors.New(cleerrors.CodeAuthenticationFailed, "Yggdrasil server returned an invalid game profile")
	}
	if len(response.AvailableProfiles) > 0 {
		found := false
		for _, available := range response.AvailableProfiles {
			if normalizeProfileID(available.ID) == profile.ID && available.Name == profile.Name {
				found = true
				break
			}
		}
		if !found {
			return yggdrasilProfile{}, cleerrors.New(cleerrors.CodeAuthenticationFailed, "selected Yggdrasil profile is not in the available profile list")
		}
	}
	return profile, nil
}

func yggdrasilUserProperties(user *yggdrasilUser) ([]models.ProfileProperty, bool, error) {
	if user == nil {
		return nil, false, nil
	}
	if len(user.Properties) > yggdrasilMaxProperties {
		return nil, true, cleerrors.New(cleerrors.CodeAuthenticationFailed, "Yggdrasil server returned too many user properties")
	}
	properties := make([]models.ProfileProperty, 0, len(user.Properties))
	for _, property := range user.Properties {
		properties = append(properties, models.ProfileProperty{
			Name: property.Name, Value: property.Value, Signature: property.Signature,
		})
	}
	if err := validateYggdrasilProfileProperties(properties); err != nil {
		return nil, true, err
	}
	return properties, true, nil
}

func validateYggdrasilProfileProperties(properties []models.ProfileProperty) error {
	if len(properties) > yggdrasilMaxProperties {
		return cleerrors.New(cleerrors.CodeAuthenticationFailed, "Yggdrasil account contains too many user properties")
	}
	totalBytes := 0
	for _, property := range properties {
		totalBytes += len(property.Name) + len(property.Value) + len(property.Signature)
		if !yggdrasilPropertyNamePattern.MatchString(property.Name) ||
			!utf8.ValidString(property.Value) || !utf8.ValidString(property.Signature) ||
			totalBytes > yggdrasilMaxPropertyBytes {
			return cleerrors.New(cleerrors.CodeAuthenticationFailed, "Yggdrasil account contains invalid user properties")
		}
	}
	return nil
}

func validateYggdrasilAccount(account models.Account) error {
	if account.Type != models.AccountYggdrasil || account.ID == "" ||
		account.CredentialRef != credentialKey(account.ID) || account.AuthServer == "" ||
		!yggdrasilProfileIDPattern.MatchString(normalizeProfileID(account.Profile.ID)) ||
		!yggdrasilProfileNamePattern.MatchString(account.Profile.Name) ||
		account.Name != account.Profile.Name ||
		normalizeProfileID(account.UUID) != normalizeProfileID(account.Profile.ID) {
		return cleerrors.New(cleerrors.CodeAuthenticationFailed, "Yggdrasil account profile is invalid")
	}
	return validateYggdrasilProfileProperties(account.Profile.Properties)
}

func validateYggdrasilServer(raw string, allowHTTP bool) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("server URL must be an absolute URL without credentials, query, or fragment")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", errors.New("server URL must use HTTPS")
	}
	if parsed.Hostname() == "" || parsed.Port() != "" && parsed.Port() != "443" && parsed.Scheme == "https" {
		if parsed.Hostname() == "" {
			return "", errors.New("server URL must include a host")
		}
	}
	if parsed.Scheme == "http" && (!allowHTTP || !isLoopbackHost(parsed.Hostname())) {
		return "", errors.New("unencrypted HTTP is permitted only for explicitly enabled loopback test servers")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func endpointURL(baseURL, endpoint string, allowHTTP bool) (string, error) {
	validated, err := validateYggdrasilServer(baseURL, allowHTTP)
	if err != nil {
		return "", err
	}
	parsed, _ := url.Parse(validated)
	path := strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(path, "/authserver") {
		path += "/authserver"
	}
	parsed.Path = path + "/" + endpoint
	return parsed.String(), nil
}

func isLoopbackHost(host string) bool {
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func decodeYggdrasilJSON(data []byte, output any) error {
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

func yggdrasilStatusError(status int, body []byte) error {
	serverCode := ""
	var response yggdrasilResponse
	if len(body) > 0 && len(body) <= yggdrasilMaxResponse && json.Unmarshal(body, &response) == nil {
		if len(response.Error) <= 64 && regexp.MustCompile(`^[A-Za-z0-9_]*$`).MatchString(response.Error) {
			serverCode = response.Error
		}
	}
	message := fmt.Sprintf("Yggdrasil server returned HTTP %d", status)
	if serverCode != "" {
		message += " (" + serverCode + ")"
	}
	return cleerrors.New(cleerrors.CodeAuthenticationFailed, message)
}

func yggdrasilServerError(code string) error {
	if len(code) > 64 || !regexp.MustCompile(`^[A-Za-z0-9_]+$`).MatchString(code) {
		code = "authentication error"
	}
	return cleerrors.New(cleerrors.CodeAuthenticationFailed, "Yggdrasil server rejected authentication ("+code+")")
}

func normalizeProfileID(id string) string {
	return strings.ToLower(strings.ReplaceAll(id, "-", ""))
}

func credentialKey(accountID string) string {
	return "yggdrasil:" + accountID
}

func newClientToken() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(value[:])
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:], nil
}

func zeroBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
