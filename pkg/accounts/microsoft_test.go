package accounts

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

const (
	testClientID     = "test-client-id"
	testOAuthAccess  = "oauth-access-token"
	testOAuthRefresh = "oauth-refresh-token"
	testXboxToken    = "xbox-token"
	testXSTSToken    = "xsts-token"
	testUHS          = "0123456789abcdef"
	testMinecraftTok = "minecraft-access-token"
	testProfileID    = "0123456789abcdef0123456789abcdef"
	testProfileName  = "PlayerName"
)

type microsoftMock struct {
	server               *httptest.Server
	pollsBeforeSuccess   int32
	pollCount            atomic.Int32
	tokenError           string
	xstsXErr             string
	entitlementNames     []string
	profileStatus        int
	oauthAccessToken     string
	oauthRefreshToken    string
	minecraftAccessToken string
}

func newMicrosoftMock(t *testing.T) *microsoftMock {
	t.Helper()
	mock := &microsoftMock{
		pollsBeforeSuccess:   2,
		entitlementNames:     []string{"game_minecraft", "product_minecraft"},
		profileStatus:        http.StatusOK,
		oauthAccessToken:     testOAuthAccess,
		oauthRefreshToken:    testOAuthRefresh,
		minecraftAccessToken: testMinecraftTok,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/devicecode", mock.handleDeviceCode)
	mux.HandleFunc("/token", mock.handleToken)
	mux.HandleFunc("/xboxlive", mock.handleXboxLive)
	mux.HandleFunc("/xsts", mock.handleXSTS)
	mux.HandleFunc("/login_with_xbox", mock.handleMinecraftLogin)
	mux.HandleFunc("/entitlements", mock.handleEntitlements)
	mux.HandleFunc("/profile", mock.handleProfile)
	mock.server = httptest.NewServer(mux)
	t.Cleanup(mock.server.Close)
	return mock
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (mock *microsoftMock) handleDeviceCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if r.FormValue("client_id") != testClientID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_client"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"device_code":      "device-code-value",
		"user_code":        "ABCD-EFGH",
		"verification_uri": "https://microsoft.com/devicelogin",
		"expires_in":       900,
		"interval":         1,
		"message":          "To sign in, use a web browser",
	})
}

func (mock *microsoftMock) handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	grantType := r.FormValue("grant_type")
	if grantType == "refresh_token" {
		if r.FormValue("refresh_token") == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"access_token":  mock.oauthAccessToken,
			"refresh_token": mock.oauthRefreshToken,
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
		return
	}
	if grantType == "urn:ietf:params:oauth:grant-type:device_code" {
		if mock.tokenError != "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": mock.tokenError})
			return
		}
		current := mock.pollCount.Add(1)
		if current < mock.pollsBeforeSuccess {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "authorization_pending"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"access_token":  mock.oauthAccessToken,
			"refresh_token": mock.oauthRefreshToken,
			"token_type":    "Bearer",
			"expires_in":    3600,
			"scope":         microsoftScope,
		})
		return
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported_grant_type"})
}

func (mock *microsoftMock) handleXboxLive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"Token": testXboxToken,
		"DisplayClaims": map[string]any{
			"xui": []map[string]string{{"uhs": testUHS}},
		},
	})
}

func (mock *microsoftMock) handleXSTS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if mock.xstsXErr != "" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"Identity": "0",
			"XErr":     mock.xstsXErr,
			"Message":  "XSTS error",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"Token": testXSTSToken,
		"DisplayClaims": map[string]any{
			"xui": []map[string]string{{"uhs": testUHS}},
		},
	})
}

func (mock *microsoftMock) handleMinecraftLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var request minecraftLoginRequest
	_ = json.NewDecoder(r.Body).Decode(&request)
	if !strings.HasPrefix(request.IdentityToken, "XBL3.0 x=") {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": mock.minecraftAccessToken,
		"token_type":   "Bearer",
		"expires_in":   86400,
	})
}

func (mock *microsoftMock) handleEntitlements(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	items := make([]map[string]string, 0, len(mock.entitlementNames))
	for _, name := range mock.entitlementNames {
		items = append(items, map[string]string{"name": name})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (mock *microsoftMock) handleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if mock.profileStatus != http.StatusOK {
		w.WriteHeader(mock.profileStatus)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":    testProfileID,
		"name":  testProfileName,
		"skins": []map[string]string{{"id": "skin-1", "state": "ACTIVE", "url": "https://example/skin", "variant": "CLASSIC"}},
		"capes": []map[string]string{},
	})
}

func (mock *microsoftMock) provider(store CredentialStore) *MicrosoftProvider {
	provider, err := NewMicrosoftProvider(MicrosoftOptions{
		ClientID:    testClientID,
		Credentials: store,
		HTTPClient:  mock.server.Client(),
		Endpoints: MicrosoftEndpoints{
			OAuthDeviceCode:       mock.server.URL + "/devicecode",
			OAuthToken:            mock.server.URL + "/token",
			XboxLive:              mock.server.URL + "/xboxlive",
			XSTS:                  mock.server.URL + "/xsts",
			MinecraftLogin:        mock.server.URL + "/login_with_xbox",
			MinecraftEntitlements: mock.server.URL + "/entitlements",
			MinecraftProfile:      mock.server.URL + "/profile",
		},
	})
	if err != nil {
		panic(err)
	}
	return provider
}

func TestMicrosoftDeviceCodeSuccess(t *testing.T) {
	mock := newMicrosoftMock(t)
	store := newMemoryCredentialStore()
	provider := mock.provider(store)
	ctx := context.Background()

	deviceCode, err := provider.StartDeviceCode(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if deviceCode.VerificationURL == "" || deviceCode.UserCode != "ABCD-EFGH" {
		t.Fatalf("device code = %+v", deviceCode)
	}

	account, err := provider.AuthenticateDeviceCode(ctx, deviceCode)
	if err != nil {
		t.Fatal(err)
	}
	if account.Type != models.AccountMicrosoft || account.Name != testProfileName || account.UUID != testProfileID {
		t.Fatalf("account = %+v", account)
	}
	if account.XUID != testUHS || account.UserType != "msa" {
		t.Fatalf("account XUID/userType = %q %q", account.XUID, account.UserType)
	}
	if account.Session.AccessToken != testMinecraftTok {
		t.Fatalf("launch session access token = %q", account.Session.AccessToken)
	}
	// Credentials must be stored per account and must not leak into the account model.
	if len(store.stored(microsoftCredentialKey(account.ID))) == 0 {
		t.Fatal("credentials were not stored")
	}
	if strings.Contains(account.CredentialRef, testOAuthRefresh) {
		t.Fatal("refresh token leaked into account credential reference")
	}
}

func TestMicrosoftDeviceCodePendingThenSuccess(t *testing.T) {
	mock := newMicrosoftMock(t)
	mock.pollsBeforeSuccess = 3
	provider := mock.provider(newMemoryCredentialStore())
	ctx := context.Background()

	deviceCode, err := provider.StartDeviceCode(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.AuthenticateDeviceCode(ctx, deviceCode); err != nil {
		t.Fatalf("AuthenticateDeviceCode = %v", err)
	}
	if mock.pollCount.Load() != 3 {
		t.Fatalf("expected 3 polls, got %d", mock.pollCount.Load())
	}
}

func TestMicrosoftDeviceCodeUserDenied(t *testing.T) {
	mock := newMicrosoftMock(t)
	mock.tokenError = "authorization_declined"
	provider := mock.provider(newMemoryCredentialStore())
	ctx := context.Background()

	deviceCode, err := provider.StartDeviceCode(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.AuthenticateDeviceCode(ctx, deviceCode)
	if err == nil || !strings.Contains(err.Error(), "declined") {
		t.Fatalf("expected declined error, got %v", err)
	}
}

func TestMicrosoftDeviceCodeExpired(t *testing.T) {
	mock := newMicrosoftMock(t)
	mock.tokenError = "expired_token"
	provider := mock.provider(newMemoryCredentialStore())
	ctx := context.Background()

	deviceCode, err := provider.StartDeviceCode(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.AuthenticateDeviceCode(ctx, deviceCode)
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expired error, got %v", err)
	}
}

func TestMicrosoftDeviceCodeCancelled(t *testing.T) {
	mock := newMicrosoftMock(t)
	mock.pollsBeforeSuccess = 100
	provider := mock.provider(newMemoryCredentialStore())

	ctx, cancel := context.WithCancel(context.Background())
	deviceCode, err := provider.StartDeviceCode(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	_, err = provider.AuthenticateDeviceCode(ctx, deviceCode)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestMicrosoftXSTSErrorClassification(t *testing.T) {
	for _, xerr := range []string{"2148916233", "2148916235", "2148916236", "2148916237", "2148916238"} {
		mock := newMicrosoftMock(t)
		mock.xstsXErr = xerr
		provider := mock.provider(newMemoryCredentialStore())
		ctx := context.Background()
		deviceCode, err := provider.StartDeviceCode(ctx)
		if err != nil {
			t.Fatal(err)
		}
		_, err = provider.AuthenticateDeviceCode(ctx, deviceCode)
		if err == nil {
			t.Fatalf("XSTS XErr %s was not rejected", xerr)
		}
	}
}

func TestMicrosoftEntitlementMissing(t *testing.T) {
	mock := newMicrosoftMock(t)
	mock.entitlementNames = []string{"product_dungeons"}
	provider := mock.provider(newMemoryCredentialStore())
	ctx := context.Background()

	deviceCode, err := provider.StartDeviceCode(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.AuthenticateDeviceCode(ctx, deviceCode)
	if err == nil || !strings.Contains(err.Error(), "does not own Minecraft") {
		t.Fatalf("expected entitlement error, got %v", err)
	}
}

func TestMicrosoftProfileNotFound(t *testing.T) {
	mock := newMicrosoftMock(t)
	mock.profileStatus = http.StatusNotFound
	provider := mock.provider(newMemoryCredentialStore())
	ctx := context.Background()

	deviceCode, err := provider.StartDeviceCode(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.AuthenticateDeviceCode(ctx, deviceCode)
	if err == nil || !strings.Contains(err.Error(), "no Minecraft: Java Edition profile") {
		t.Fatalf("expected profile-not-found error, got %v", err)
	}
}

func TestMicrosoftRefreshAndResolveLaunchAccount(t *testing.T) {
	mock := newMicrosoftMock(t)
	store := newMemoryCredentialStore()
	provider := mock.provider(store)
	ctx := context.Background()

	deviceCode, err := provider.StartDeviceCode(ctx)
	if err != nil {
		t.Fatal(err)
	}
	account, err := provider.AuthenticateDeviceCode(ctx, deviceCode)
	if err != nil {
		t.Fatal(err)
	}
	account.Session = models.AccountSession{}

	// Force a fresh Minecraft token by expiring the cached session.
	resolved, err := provider.ResolveLaunchAccount(ctx, account)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Session.AccessToken != testMinecraftTok {
		t.Fatalf("resolved session access token = %q", resolved.Session.AccessToken)
	}

	refreshed, err := provider.Refresh(ctx, account)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Name != testProfileName {
		t.Fatalf("refreshed account = %+v", refreshed)
	}
}

func TestMicrosoftValidateRejectsAccountWithoutCredentialRef(t *testing.T) {
	provider := newMicrosoftMock(t).provider(newMemoryCredentialStore())
	account := models.Account{
		ID: "account-id", Type: models.AccountMicrosoft,
		Name: testProfileName, UUID: testProfileID,
		Profile: models.GameProfile{ID: testProfileID, Name: testProfileName},
	}
	if err := provider.Validate(context.Background(), account); err == nil {
		t.Fatal("Validate accepted an account without a valid credential reference")
	}
}

func TestMicrosoftRequiresClientID(t *testing.T) {
	provider, err := NewMicrosoftProvider(MicrosoftOptions{
		Credentials: newMemoryCredentialStore(),
		HTTPClient:  &http.Client{Timeout: time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.StartDeviceCode(context.Background()); err == nil {
		t.Fatal("StartDeviceCode succeeded without a client ID")
	}
}

func TestMicrosoftProfileIDIsNormalized(t *testing.T) {
	account := models.Account{
		ID: "id", Type: models.AccountMicrosoft, Name: testProfileName,
		UUID:          strings.ToUpper(testProfileID),
		Profile:       models.GameProfile{ID: testProfileID, Name: testProfileName},
		CredentialRef: microsoftCredentialKey("id"),
	}
	if err := validateMicrosoftAccount(account); err != nil {
		t.Fatalf("validateMicrosoftAccount = %v", err)
	}
}
