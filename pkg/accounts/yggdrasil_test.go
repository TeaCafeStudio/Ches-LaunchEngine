package accounts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

const (
	testYggdrasilProfileID = "123456781234123412341234567890ab"
	testYggdrasilUsername  = "GamePlayer"
)

type memoryCredentialStore struct {
	mu        sync.Mutex
	values    map[string][]byte
	err       error
	deleteErr error
}

func newMemoryCredentialStore() *memoryCredentialStore {
	return &memoryCredentialStore{values: make(map[string][]byte)}
}

func (store *memoryCredentialStore) Store(ctx context.Context, key string, value []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.err != nil {
		return store.err
	}
	store.values[key] = append([]byte(nil), value...)
	return nil
}

func (store *memoryCredentialStore) Retrieve(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.err != nil {
		return nil, store.err
	}
	value, exists := store.values[key]
	if !exists {
		return nil, errors.New("missing")
	}
	return append([]byte(nil), value...), nil
}

func (store *memoryCredentialStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.err != nil {
		return store.err
	}
	if store.deleteErr != nil {
		return store.deleteErr
	}
	delete(store.values, key)
	return nil
}

func (store *memoryCredentialStore) stored(key string) []byte {
	store.mu.Lock()
	defer store.mu.Unlock()
	return append([]byte(nil), store.values[key]...)
}

func newTestYggdrasilProvider(t *testing.T, server *httptest.Server, store CredentialStore) *YggdrasilProvider {
	t.Helper()
	provider, err := NewYggdrasilProvider(YggdrasilOptions{
		Credentials: store, HTTPClient: server.Client(), AllowHTTP: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return provider
}

func TestYggdrasilAuthenticateRefreshValidateAndInvalidate(t *testing.T) {
	store := newMemoryCredentialStore()
	var mu sync.Mutex
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		paths = append(paths, request.URL.Path)
		mu.Unlock()
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Error(err)
			return
		}
		switch request.URL.Path {
		case "/authserver/authenticate":
			var payload yggdrasilAuthenticateRequest
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Error(err)
				return
			}
			if payload.Agent != (yggdrasilAgent{Name: "Minecraft", Version: 1}) ||
				payload.Username != "login@example.test" || payload.Password != "password-never-log" ||
				payload.ClientToken == "" || !payload.RequestUser {
				t.Errorf("unexpected authenticate payload: %+v", payload)
			}
			writeYggdrasilResponseWithProperties(writer, payload.ClientToken, "access-one", []yggdrasilUserProperty{
				{Name: "textures", Value: "texture-data", Signature: "texture-signature-one"},
			})
		case "/authserver/refresh":
			var payload yggdrasilTokenRequest
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Error(err)
				return
			}
			if payload.AccessToken != "access-one" || payload.SelectedProfile == nil ||
				payload.SelectedProfile.ID != testYggdrasilProfileID || !payload.RequestUser {
				t.Errorf("unexpected refresh payload: %+v", payload)
			}
			writeYggdrasilResponseWithProperties(writer, payload.ClientToken, "access-two", []yggdrasilUserProperty{
				{Name: "textures", Value: "texture-data", Signature: "texture-signature-two"},
			})
		case "/authserver/validate":
			var payload yggdrasilTokenRequest
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Error(err)
				return
			}
			if payload.AccessToken != "access-one" && payload.AccessToken != "access-two" {
				t.Errorf("unexpected validate access token: %q", payload.AccessToken)
			}
			writer.WriteHeader(http.StatusNoContent)
		case "/authserver/invalidate":
			var payload yggdrasilTokenRequest
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Error(err)
				return
			}
			if payload.AccessToken != "access-two" || payload.ClientToken == "" {
				t.Errorf("unexpected invalidate payload: %+v", payload)
			}
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	provider := newTestYggdrasilProvider(t, server, store)
	account, err := provider.Authenticate(context.Background(), models.AccountAuthRequest{
		Type: models.AccountYggdrasil, AuthServer: server.URL,
		Username: "login@example.test", Password: "password-never-log",
	})
	if err != nil {
		t.Fatal(err)
	}
	if account.Type != models.AccountYggdrasil || account.Name != testYggdrasilUsername ||
		account.UUID != testYggdrasilProfileID || account.CredentialRef == "" {
		t.Fatalf("unexpected account: %+v", account)
	}
	if len(account.Profile.Properties) != 1 || account.Profile.Properties[0].Signature != "texture-signature-one" {
		t.Fatalf("authentication discarded Yggdrasil user properties: %+v", account.Profile.Properties)
	}
	serialized, err := json.Marshal(account)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(serialized), "access-one") || strings.Contains(string(serialized), "password-never-log") {
		t.Fatalf("account JSON contains credentials: %s", serialized)
	}
	if err := provider.Validate(context.Background(), account); err != nil {
		t.Fatal(err)
	}
	refreshed, err := provider.Refresh(context.Background(), account)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Profile.ID != account.Profile.ID || len(refreshed.Profile.Properties) != 1 ||
		refreshed.Profile.Properties[0].Signature != "texture-signature-two" {
		t.Fatalf("refresh changed selected profile: %+v", refreshed.Profile)
	}
	if err := provider.Validate(context.Background(), refreshed); err != nil {
		t.Fatal(err)
	}
	var persisted yggdrasilSession
	if err := json.Unmarshal(store.stored(account.CredentialRef), &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.AccessToken != "access-two" || persisted.ClientToken == "" {
		t.Fatalf("refreshed credentials were not persisted: %+v", persisted)
	}
	if err := provider.Invalidate(context.Background(), refreshed); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Retrieve(context.Background(), account.CredentialRef); err == nil {
		t.Fatal("invalidate did not remove the stored credentials")
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{
		"/authserver/authenticate", "/authserver/validate",
		"/authserver/refresh", "/authserver/validate", "/authserver/invalidate",
	}
	if fmt.Sprint(paths) != fmt.Sprint(want) {
		t.Fatalf("request paths = %v, want %v", paths, want)
	}
}

func TestYggdrasilSignoutUsesLoginUsernameAndDeletesCredentials(t *testing.T) {
	store := newMemoryCredentialStore()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/authserver/authenticate" {
			var payload yggdrasilAuthenticateRequest
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Error(err)
				return
			}
			writeYggdrasilResponse(writer, payload.ClientToken, "access-one")
			return
		}
		if request.URL.Path != "/authserver/signout" {
			http.NotFound(writer, request)
			return
		}
		var payload yggdrasilSignoutRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Error(err)
			return
		}
		if payload.Username != "login@example.test" || payload.Password != "signout-password" {
			t.Errorf("signout used wrong credentials: %+v", payload)
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	provider := newTestYggdrasilProvider(t, server, store)
	account, err := provider.Authenticate(context.Background(), models.AccountAuthRequest{
		Type: models.AccountYggdrasil, AuthServer: server.URL,
		Username: "login@example.test", Password: "login-password",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := provider.Signout(context.Background(), account, "signout-password"); err != nil {
		t.Fatal(err)
	}
	if len(store.stored(account.CredentialRef)) != 0 {
		t.Fatal("signout did not remove stored credentials")
	}
}

func TestYggdrasilRejectsBadResponsesAndUnsafeServers(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		body        string
		wantMessage string
	}{
		{name: "invalid json", status: http.StatusOK, body: "not-json", wantMessage: "invalid response"},
		{name: "server error", status: http.StatusUnauthorized, body: `{"error":"ForbiddenOperationException","errorMessage":"bad password"}`, wantMessage: "HTTP 401"},
		{name: "missing profile", status: http.StatusOK, body: `{"accessToken":"token","clientToken":"replace","availableProfiles":[]}`, wantMessage: "did not select"},
		{name: "profile mismatch", status: http.StatusOK, body: `{"accessToken":"token","clientToken":"replace","availableProfiles":[{"id":"ffffffffffffffffffffffffffffffff","name":"Other"}],"selectedProfile":{"id":"123456781234123412341234567890ab","name":"GamePlayer"}}`, wantMessage: "not in the available"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				var authenticateRequest yggdrasilAuthenticateRequest
				if err := json.NewDecoder(request.Body).Decode(&authenticateRequest); err != nil {
					t.Error(err)
					return
				}
				writer.WriteHeader(test.status)
				_, _ = io.WriteString(writer, strings.ReplaceAll(test.body, "replace", authenticateRequest.ClientToken))
			}))
			defer server.Close()
			provider := newTestYggdrasilProvider(t, server, newMemoryCredentialStore())
			_, err := provider.Authenticate(context.Background(), models.AccountAuthRequest{
				Type: models.AccountYggdrasil, AuthServer: server.URL,
				Username: "login", Password: "sensitive-password",
			})
			if err == nil || !strings.Contains(err.Error(), test.wantMessage) {
				t.Fatalf("Authenticate() error = %v, want message containing %q", err, test.wantMessage)
			}
			if strings.Contains(err.Error(), "sensitive-password") || strings.Contains(err.Error(), "bad password") {
				t.Fatalf("error exposed credentials or raw server message: %v", err)
			}
		})
	}
	for _, raw := range []string{
		"http://example.com", "http://127.0.0.1:1234", "https://user:pass@example.com",
		"https://example.com?token=secret", "file:///etc/passwd",
	} {
		if _, err := validateYggdrasilServer(raw, false); err == nil {
			t.Errorf("validateYggdrasilServer(%q) succeeded", raw)
		}
	}
	if _, err := validateYggdrasilServer("http://127.0.0.1:1234", true); err != nil {
		t.Fatalf("explicit loopback HTTP should be allowed for local test servers: %v", err)
	}
	if _, err := validateYggdrasilServer("http://example.com", true); err == nil {
		t.Fatal("explicit HTTP option allowed an external plaintext server")
	}
}

func TestYggdrasilHonorsContextTimeoutAndCredentialStoreErrors(t *testing.T) {
	store := newMemoryCredentialStore()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/authserver/authenticate" {
			time.Sleep(50 * time.Millisecond)
			return
		}
		http.NotFound(writer, request)
	}))
	defer server.Close()
	provider := newTestYggdrasilProvider(t, server, store)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	_, err := provider.Authenticate(ctx, models.AccountAuthRequest{
		Type: models.AccountYggdrasil, AuthServer: server.URL,
		Username: "login", Password: "password",
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Authenticate timeout error = %v", err)
	}

	failingStore := newMemoryCredentialStore()
	failingStore.err = errors.New("keyring unavailable")
	workingServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload yggdrasilAuthenticateRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Error(err)
			return
		}
		writeYggdrasilResponse(writer, payload.ClientToken, "private-token")
	}))
	defer workingServer.Close()
	provider = newTestYggdrasilProvider(t, workingServer, failingStore)
	_, err = provider.Authenticate(context.Background(), models.AccountAuthRequest{
		Type: models.AccountYggdrasil, AuthServer: workingServer.URL,
		Username: "login", Password: "password",
	})
	if err == nil || !strings.Contains(err.Error(), "store Yggdrasil credentials securely") ||
		strings.Contains(err.Error(), "private-token") || strings.Contains(err.Error(), "password") {
		t.Fatalf("credential-store failure was hidden or exposed secrets: %v", err)
	}
}

func TestYggdrasilHandlesTLSAndInterruptedConnections(t *testing.T) {
	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		t.Error("request unexpectedly bypassed TLS certificate validation")
	}))
	defer tlsServer.Close()
	provider, err := NewYggdrasilProvider(YggdrasilOptions{Credentials: newMemoryCredentialStore()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.Authenticate(context.Background(), models.AccountAuthRequest{
		Type: models.AccountYggdrasil, AuthServer: tlsServer.URL,
		Username: "login", Password: "password-secret",
	})
	if err == nil || strings.Contains(err.Error(), "password-secret") {
		t.Fatalf("TLS verification error was missing or exposed credentials: %v", err)
	}

	interrupted := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		connection, _, hijackErr := writer.(http.Hijacker).Hijack()
		if hijackErr != nil {
			t.Errorf("hijack connection: %v", hijackErr)
			return
		}
		_ = connection.Close()
	}))
	defer interrupted.Close()
	provider = newTestYggdrasilProvider(t, interrupted, newMemoryCredentialStore())
	_, err = provider.Authenticate(context.Background(), models.AccountAuthRequest{
		Type: models.AccountYggdrasil, AuthServer: interrupted.URL,
		Username: "login", Password: "password-secret",
	})
	if err == nil || strings.Contains(err.Error(), "password-secret") {
		t.Fatalf("interrupted connection error was missing or exposed credentials: %v", err)
	}
}

func TestYggdrasilRejectsRedirectsAndOversizedResponses(t *testing.T) {
	var redirected atomic.Bool
	destination := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		redirected.Store(true)
		writeYggdrasilResponse(writer, "unexpected", "unexpected")
	}))
	defer destination.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, destination.URL+"/authserver/authenticate", http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	provider := newTestYggdrasilProvider(t, redirect, newMemoryCredentialStore())
	_, err := provider.Authenticate(context.Background(), models.AccountAuthRequest{
		Type: models.AccountYggdrasil, AuthServer: redirect.URL,
		Username: "login", Password: "password",
	})
	if err == nil || !strings.Contains(err.Error(), "HTTP 307") || redirected.Load() {
		t.Fatalf("redirect was followed or not reported: error=%v redirected=%v", err, redirected.Load())
	}

	oversized := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, strings.Repeat("x", yggdrasilMaxResponse+1))
	}))
	defer oversized.Close()
	provider = newTestYggdrasilProvider(t, oversized, newMemoryCredentialStore())
	_, err = provider.Authenticate(context.Background(), models.AccountAuthRequest{
		Type: models.AccountYggdrasil, AuthServer: oversized.URL,
		Username: "login", Password: "password",
	})
	if err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("oversized response error = %v", err)
	}
}

func TestYggdrasilRefreshFailureDoesNotReplaceCredentials(t *testing.T) {
	store := newMemoryCredentialStore()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/authserver/authenticate":
			var payload yggdrasilAuthenticateRequest
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Error(err)
				return
			}
			writeYggdrasilResponse(writer, payload.ClientToken, "access-one")
		case "/authserver/refresh":
			writer.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(writer, `{"error":"ForbiddenOperationException","errorMessage":"expired"}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	provider := newTestYggdrasilProvider(t, server, store)
	account, err := provider.Authenticate(context.Background(), models.AccountAuthRequest{
		Type: models.AccountYggdrasil, AuthServer: server.URL,
		Username: "login", Password: "password",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Refresh(context.Background(), account); err == nil {
		t.Fatal("Refresh succeeded despite server rejection")
	}
	var persisted yggdrasilSession
	if err := json.Unmarshal(store.stored(account.CredentialRef), &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.AccessToken != "access-one" {
		t.Fatalf("failed refresh replaced credentials: %+v", persisted)
	}
}

func TestResolveSessionRefreshesOnlyRejectedAccessTokens(t *testing.T) {
	tests := []struct {
		name                string
		validateStatus      int
		wantAccessToken     string
		wantRefreshRequests int
	}{
		{name: "expired token is refreshed", validateStatus: http.StatusForbidden, wantAccessToken: "access-two", wantRefreshRequests: 1},
		{name: "server error is not retried as refresh", validateStatus: http.StatusInternalServerError, wantRefreshRequests: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newMemoryCredentialStore()
			var refreshRequests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case "/authserver/authenticate":
					var payload yggdrasilAuthenticateRequest
					if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
						t.Error(err)
						return
					}
					writeYggdrasilResponse(writer, payload.ClientToken, "access-one")
				case "/authserver/validate":
					var payload yggdrasilTokenRequest
					if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
						t.Error(err)
						return
					}
					status := test.validateStatus
					if payload.AccessToken == "access-two" {
						status = http.StatusNoContent
					}
					writer.WriteHeader(status)
				case "/authserver/refresh":
					refreshRequests.Add(1)
					var payload yggdrasilTokenRequest
					if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
						t.Error(err)
						return
					}
					if payload.AccessToken != "access-one" {
						t.Errorf("refresh received access token %q", payload.AccessToken)
					}
					writeYggdrasilResponse(writer, payload.ClientToken, "access-two")
				default:
					http.NotFound(writer, request)
				}
			}))
			defer server.Close()
			provider := newTestYggdrasilProvider(t, server, store)
			account, err := provider.Authenticate(context.Background(), models.AccountAuthRequest{
				Type: models.AccountYggdrasil, AuthServer: server.URL,
				Username: "login", Password: "password",
			})
			if err != nil {
				t.Fatal(err)
			}
			session, err := provider.ResolveSession(context.Background(), account)
			if test.wantAccessToken == "" {
				if err == nil || session.AccessToken != "" {
					t.Fatalf("ResolveSession() = %+v, %v; want validation error", session, err)
				}
			} else if err != nil || session.AccessToken != test.wantAccessToken {
				t.Fatalf("ResolveSession() = %+v, %v; want token %q", session, err, test.wantAccessToken)
			}
			if int(refreshRequests.Load()) != test.wantRefreshRequests {
				t.Fatalf("refresh requests = %d, want %d", refreshRequests.Load(), test.wantRefreshRequests)
			}
		})
	}
}

func TestYggdrasilSerializesConcurrentRefreshes(t *testing.T) {
	store := newMemoryCredentialStore()
	refreshStarted := make(chan string, 2)
	releaseFirst := make(chan struct{})
	var releaseFirstOnce sync.Once
	release := func() { releaseFirstOnce.Do(func() { close(releaseFirst) }) }
	defer release()
	var refreshCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/authserver/authenticate":
			var payload yggdrasilAuthenticateRequest
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Error(err)
				return
			}
			writeYggdrasilResponse(writer, payload.ClientToken, "access-one")
		case "/authserver/refresh":
			var payload yggdrasilTokenRequest
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Error(err)
				return
			}
			refreshStarted <- payload.AccessToken
			count := refreshCount.Add(1)
			if count == 1 {
				<-releaseFirst
			}
			nextToken := "access-two"
			if count == 2 {
				nextToken = "access-three"
			}
			writeYggdrasilResponse(writer, payload.ClientToken, nextToken)
		case "/authserver/validate":
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	provider := newTestYggdrasilProvider(t, server, store)
	account, err := provider.Authenticate(context.Background(), models.AccountAuthRequest{
		Type: models.AccountYggdrasil, AuthServer: server.URL, Username: "login", Password: "password",
	})
	if err != nil {
		t.Fatal(err)
	}

	results := make(chan error, 2)
	go func() {
		_, err := provider.Refresh(context.Background(), account)
		results <- err
	}()
	if token := <-refreshStarted; token != "access-one" {
		t.Fatalf("first refresh token = %q, want access-one", token)
	}
	go func() {
		_, err := provider.Refresh(context.Background(), account)
		results <- err
	}()
	select {
	case token := <-refreshStarted:
		t.Fatalf("second refresh bypassed account lock with token %q", token)
	case <-time.After(25 * time.Millisecond):
	}
	release()
	if err := <-results; err != nil {
		t.Fatal(err)
	}
	if err := <-results; err != nil {
		t.Fatal(err)
	}
	if token := <-refreshStarted; token != "access-two" {
		t.Fatalf("second refresh token = %q, want access-two", token)
	}
	var persisted yggdrasilSession
	if err := json.Unmarshal(store.stored(account.CredentialRef), &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.AccessToken != "access-three" {
		t.Fatalf("concurrent refreshes left token %q, want access-three", persisted.AccessToken)
	}
}

func TestManagerPersistsYggdrasilProfileSeparatelyFromCredentials(t *testing.T) {
	store := newMemoryCredentialStore()
	validateCount := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/authserver/authenticate":
			var payload yggdrasilAuthenticateRequest
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Error(err)
				return
			}
			writeYggdrasilResponseWithProperties(writer, payload.ClientToken, "private-access-token", []yggdrasilUserProperty{
				{Name: "textures", Value: "texture-data", Signature: "texture-signature"},
			})
		case "/authserver/refresh":
			var payload yggdrasilTokenRequest
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Error(err)
				return
			}
			writeYggdrasilResponseWithProperties(writer, payload.ClientToken, "refreshed-access-token", []yggdrasilUserProperty{
				{Name: "textures", Value: "refreshed-texture-data", Signature: "refreshed-texture-signature"},
			})
		case "/authserver/validate":
			validateCount++
			if validateCount == 2 {
				writer.WriteHeader(http.StatusForbidden)
				return
			}
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	provider, err := NewYggdrasilProvider(YggdrasilOptions{
		Credentials: store, HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	accountPath := t.TempDir() + "/accounts.json"
	manager, err := NewManager(accountPath, provider)
	if err != nil {
		t.Fatal(err)
	}
	account, err := manager.AuthenticateAccount(context.Background(), models.AccountAuthRequest{
		Type: models.AccountYggdrasil, AuthServer: server.URL,
		Username: "login", Password: "password",
	})
	if err != nil {
		t.Fatal(err)
	}
	fileBytes, err := os.ReadFile(accountPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(fileBytes), "private-access-token") || strings.Contains(string(fileBytes), "password") {
		t.Fatalf("account JSON persisted credentials: %s", fileBytes)
	}
	resolved, err := ResolveLaunchAccount(context.Background(), manager, models.Account{ID: account.ID})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Session.AccessToken != "refreshed-access-token" ||
		resolved.Session.ClientToken == "" {
		t.Fatalf("launch account session was not resolved: %+v", resolved.Session)
	}
	if len(resolved.Profile.Properties) != 1 || resolved.Profile.Properties[0].Signature != "refreshed-texture-signature" ||
		resolved.Profile.Properties[0].Value != "refreshed-texture-data" {
		t.Fatalf("resolved launch account lost refreshed Yggdrasil data: profile=%+v", resolved.Profile)
	}
	reloaded, err := NewManager(accountPath, provider)
	if err != nil {
		t.Fatal(err)
	}
	reloadedAccount, err := reloaded.GetAccount(context.Background(), account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloadedAccount.Profile.Properties) != 1 ||
		reloadedAccount.Profile.Properties[0].Signature != "refreshed-texture-signature" {
		t.Fatalf("refreshed profile was not persisted: %+v", reloadedAccount.Profile)
	}
	if err := reloaded.RemoveAccount(context.Background(), account.ID); err != nil {
		t.Fatal(err)
	}
	if len(store.stored(account.CredentialRef)) != 0 {
		t.Fatal("removing account did not clean up credentials")
	}
}

func TestYggdrasilCredentialsAreIsolatedBetweenAccounts(t *testing.T) {
	store := newMemoryCredentialStore()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/authserver/authenticate":
			var payload yggdrasilAuthenticateRequest
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Error(err)
				return
			}
			writeYggdrasilResponse(writer, payload.ClientToken, payload.Username+"-access")
		case "/authserver/validate":
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	provider := newTestYggdrasilProvider(t, server, store)
	manager, err := NewManager("", provider)
	if err != nil {
		t.Fatal(err)
	}
	accounts := make([]models.Account, 2)
	for index, username := range []string{"login-one", "login-two"} {
		accounts[index], err = manager.AuthenticateAccount(context.Background(), models.AccountAuthRequest{
			Type: models.AccountYggdrasil, AuthServer: server.URL, Username: username, Password: "password",
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if accounts[0].CredentialRef == accounts[1].CredentialRef {
		t.Fatal("separate accounts share a credential reference")
	}
	for index, want := range []string{"login-one-access", "login-two-access"} {
		var session yggdrasilSession
		if err := json.Unmarshal(store.stored(accounts[index].CredentialRef), &session); err != nil {
			t.Fatal(err)
		}
		if session.AccessToken != want {
			t.Fatalf("account %d access token = %q, want %q", index, session.AccessToken, want)
		}
	}
	if err := manager.RemoveAccount(context.Background(), accounts[0].ID); err != nil {
		t.Fatal(err)
	}
	if len(store.stored(accounts[0].CredentialRef)) != 0 || len(store.stored(accounts[1].CredentialRef)) == 0 {
		t.Fatal("removing one account affected the other account's credentials")
	}
}

func TestManagerCleansCredentialsWhenValidationRejectsAuthentication(t *testing.T) {
	store := newMemoryCredentialStore()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/authserver/authenticate":
			var payload yggdrasilAuthenticateRequest
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Error(err)
				return
			}
			writeYggdrasilResponse(writer, payload.ClientToken, "private-token")
		case "/authserver/validate":
			writer.WriteHeader(http.StatusForbidden)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	provider := newTestYggdrasilProvider(t, server, store)
	manager, err := NewManager("", provider)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.AuthenticateAccount(context.Background(), models.AccountAuthRequest{
		Type: models.AccountYggdrasil, AuthServer: server.URL,
		Username: "login", Password: "password",
	}); err == nil {
		t.Fatal("AuthenticateAccount succeeded despite failed session validation")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.values) != 0 {
		t.Fatalf("rejected authentication left credentials stored: %d", len(store.values))
	}
}

func TestManagerRemoveDoesNotDeadlockWithRefresh(t *testing.T) {
	store := newMemoryCredentialStore()
	refreshStarted := make(chan struct{})
	releaseRefresh := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseRefresh) }) }
	defer release()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/authserver/authenticate":
			var payload yggdrasilAuthenticateRequest
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Error(err)
				return
			}
			writeYggdrasilResponse(writer, payload.ClientToken, "access-one")
		case "/authserver/validate":
			writer.WriteHeader(http.StatusNoContent)
		case "/authserver/refresh":
			close(refreshStarted)
			<-releaseRefresh
			var payload yggdrasilTokenRequest
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Error(err)
				return
			}
			writeYggdrasilResponse(writer, payload.ClientToken, "access-two")
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	provider := newTestYggdrasilProvider(t, server, store)
	manager, err := NewManager(filepath.Join(t.TempDir(), "accounts.json"), provider)
	if err != nil {
		t.Fatal(err)
	}
	account, err := manager.AuthenticateAccount(context.Background(), models.AccountAuthRequest{
		Type: models.AccountYggdrasil, AuthServer: server.URL, Username: "login", Password: "password",
	})
	if err != nil {
		t.Fatal(err)
	}

	refreshDone := make(chan error, 1)
	go func() {
		_, err := manager.RefreshAccount(context.Background(), account.ID)
		refreshDone <- err
	}()
	<-refreshStarted
	removeDone := make(chan error, 1)
	go func() {
		removeDone <- manager.RemoveAccount(context.Background(), account.ID)
	}()
	select {
	case <-removeDone:
		t.Fatal("account removal completed before in-flight refresh released the credential lock")
	case <-time.After(25 * time.Millisecond):
	}
	release()
	if err := <-refreshDone; err != nil {
		t.Fatalf("in-flight refresh failed before account removal completed: %v", err)
	}
	if err := <-removeDone; err != nil {
		t.Fatal(err)
	}
	if _, err := manager.GetAccount(context.Background(), account.ID); err == nil {
		t.Fatal("removed account remains in the manager")
	}
	if len(store.stored(account.CredentialRef)) != 0 {
		t.Fatal("removed account credentials remain in the credential store")
	}
}

func TestManagerReportsCredentialCleanupFailureAfterRemovingProfile(t *testing.T) {
	store := newMemoryCredentialStore()
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/authserver/authenticate":
			var payload yggdrasilAuthenticateRequest
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Error(err)
				return
			}
			writeYggdrasilResponse(writer, payload.ClientToken, "private-token")
		case "/authserver/validate":
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	provider, err := NewYggdrasilProvider(YggdrasilOptions{
		Credentials: store, HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	accountPath := t.TempDir() + "/accounts.json"
	manager, err := NewManager(accountPath, provider)
	if err != nil {
		t.Fatal(err)
	}
	account, err := manager.AuthenticateAccount(context.Background(), models.AccountAuthRequest{
		Type: models.AccountYggdrasil, AuthServer: server.URL,
		Username: "login", Password: "password",
	})
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	store.deleteErr = errors.New("keyring unavailable")
	store.mu.Unlock()
	if err := manager.RemoveAccount(context.Background(), account.ID); err == nil {
		t.Fatal("RemoveAccount hid a credential cleanup failure")
	}
	if _, err := manager.GetAccount(context.Background(), account.ID); err == nil {
		t.Fatal("account remained in manager after its profile was removed from persistent storage")
	}
	if len(store.stored(account.CredentialRef)) == 0 {
		t.Fatal("credential unexpectedly disappeared despite a delete failure")
	}
	reloaded, err := NewManager(accountPath, provider)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reloaded.GetAccount(context.Background(), account.ID); err == nil {
		t.Fatal("removed account remained in persistent storage")
	}
}

func writeYggdrasilResponse(writer http.ResponseWriter, clientToken, accessToken string) {
	writeYggdrasilResponseWithProperties(writer, clientToken, accessToken, nil)
}

func writeYggdrasilResponseWithProperties(writer http.ResponseWriter, clientToken, accessToken string, properties []yggdrasilUserProperty) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"accessToken":       accessToken,
		"clientToken":       clientToken,
		"availableProfiles": []yggdrasilProfile{{ID: testYggdrasilProfileID, Name: testYggdrasilUsername}},
		"selectedProfile":   yggdrasilProfile{ID: testYggdrasilProfileID, Name: testYggdrasilUsername},
		"user":              yggdrasilUser{ID: "account-id", Properties: properties},
	})
}
