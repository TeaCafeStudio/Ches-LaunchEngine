package accounts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/core"
	cleerrors "github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/errors"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

func TestOfflineAccountLifecycleAndPersistence(t *testing.T) {
	ctx := context.Background()
	storePath := filepath.Join(t.TempDir(), "accounts.json")
	manager, err := NewManager(storePath)
	if err != nil {
		t.Fatal(err)
	}
	account, err := manager.AuthenticateAccount(ctx, models.AccountAuthRequest{
		Type: models.AccountOffline, Username: "Notch",
	})
	if err != nil {
		t.Fatal(err)
	}
	if account.UUID != "b50ad385-829d-3141-a216-7e7d7539ba7f" {
		t.Fatalf("generated UUID = %q", account.UUID)
	}
	profile, err := manager.GetAccountProfile(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Name != "Notch" || profile.ID != account.UUID {
		t.Fatalf("unexpected profile: %+v", profile)
	}
	if err := manager.ValidateAccount(ctx, account.ID); err != nil {
		t.Fatal(err)
	}
	active, err := manager.ActiveAccount(ctx)
	if err != nil || active.ID != account.ID {
		t.Fatalf("ActiveAccount() = %+v, %v", active, err)
	}
	persisted, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(persisted), "accessToken") || strings.Contains(string(persisted), "refreshToken") {
		t.Fatalf("account profile store contains token fields: %s", persisted)
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(storePath); err != nil {
			t.Fatal(err)
		} else if info.Mode().Perm()&0077 != 0 {
			t.Fatalf("account store permissions are too broad: %o", info.Mode().Perm())
		}
	}
	reloaded, err := NewManager(storePath)
	if err != nil {
		t.Fatal(err)
	}
	loadedAccount, err := reloaded.GetAccount(ctx, account.ID)
	if err != nil || loadedAccount.UUID != account.UUID {
		t.Fatalf("GetAccount() after reload = %+v, %v", loadedAccount, err)
	}
	if err := reloaded.RemoveAccount(ctx, account.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := reloaded.GetAccount(ctx, account.ID); err == nil {
		t.Fatal("removed account still exists")
	}
	if _, err := reloaded.ActiveAccount(ctx); err == nil {
		t.Fatal("removing the active account did not clear the active selection")
	}
}

func TestOfflineAccountCustomUUIDAndValidation(t *testing.T) {
	provider := NewOfflineProvider()
	account, err := provider.Authenticate(context.Background(), models.AccountAuthRequest{
		Type: models.AccountOffline, Username: "Player_1",
		UUID: "12345678-1234-1234-1234-123456789ABC",
	})
	if err != nil {
		t.Fatal(err)
	}
	if account.UUID != "12345678-1234-1234-1234-123456789abc" {
		t.Fatalf("custom UUID was not normalized: %s", account.UUID)
	}
	if err := provider.Validate(context.Background(), account); err != nil {
		t.Fatal(err)
	}
	for _, request := range []models.AccountAuthRequest{
		{Type: models.AccountOffline, Username: "bad name"},
		{Type: models.AccountOffline, Username: "Player", UUID: "not-a-uuid"},
	} {
		if _, err := provider.Authenticate(context.Background(), request); err == nil {
			t.Fatalf("Authenticate(%+v) succeeded", request)
		}
	}
	if _, err := provider.Refresh(context.Background(), account); !errors.Is(err, core.ErrAccountOperationUnsupported) {
		t.Fatalf("offline refresh error = %v", err)
	}
	if err := provider.Logout(context.Background(), account); !errors.Is(err, core.ErrAccountOperationUnsupported) {
		t.Fatalf("offline logout error = %v", err)
	}
}

func TestAccountProvidersAreRegistered(t *testing.T) {
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	for _, accountType := range []models.AccountType{models.AccountOffline, models.AccountYggdrasil, models.AccountMicrosoft} {
		if _, err := manager.Provider(accountType); err != nil {
			t.Fatalf("Provider(%s): %v", accountType, err)
		}
	}
	if _, err := manager.Provider(models.AccountType("unknown")); !errors.Is(err, core.ErrAccountOperationUnsupported) {
		t.Fatalf("Provider(unknown) error = %v", err)
	}
	if err := manager.RegisterProvider(NewOfflineProvider()); err == nil {
		t.Fatal("duplicate provider registration succeeded")
	}
	// Microsoft authentication is interactive and must use the device code flow.
	_, err = manager.AuthenticateAccount(context.Background(), models.AccountAuthRequest{Type: models.AccountMicrosoft})
	var engineError *cleerrors.Error
	if !errors.As(err, &engineError) || engineError.Code != cleerrors.CodeAuthenticationFailed {
		t.Fatalf("Microsoft AuthenticateAccount error = %v", err)
	}
}

func TestAccountStoreRejectsMalformedAndUnsupportedRecords(t *testing.T) {
	for _, store := range []string{
		`{"version":1,"accounts":[{"id":"x","type":"microsoft","name":"x","uuid":"12345678-1234-1234-1234-123456789012","profile":{"id":"12345678-1234-1234-1234-123456789012","name":"x"}}]}`,
		`{"version":99,"accounts":[]}`,
		`{"version":1,"accounts":[],"accessToken":"must-not-be-persisted"}`,
		`{invalid`,
	} {
		path := filepath.Join(t.TempDir(), "accounts.json")
		if err := os.WriteFile(path, []byte(store), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := NewManager(path); err == nil {
			t.Fatalf("NewManager accepted invalid store %q", store)
		}
	}
}

func TestSessionTokensAreNeverJSONSerialized(t *testing.T) {
	data, err := json.Marshal(models.AccountSession{
		AccessToken: "access-secret", RefreshToken: "refresh-secret",
		ClientID: "client-id",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret") || strings.Contains(string(data), "client-id") {
		t.Fatalf("session data was serialized: %s", data)
	}
	requestData, err := json.Marshal(models.AccountAuthRequest{Username: "user", Password: "password-secret"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(requestData), "password-secret") {
		t.Fatalf("authentication request serialized its password: %s", requestData)
	}
}

func TestSetActiveAccountAndContextCancellation(t *testing.T) {
	ctx := context.Background()
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	first, err := manager.AuthenticateAccount(ctx, models.AccountAuthRequest{Type: models.AccountOffline, Username: "First"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.AuthenticateAccount(ctx, models.AccountAuthRequest{Type: models.AccountOffline, Username: "Second"})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.SetActiveAccount(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	active, err := manager.ActiveAccount(ctx)
	if err != nil || active.ID != second.ID {
		t.Fatalf("ActiveAccount() = %+v, %v", active, err)
	}
	if err := manager.RemoveAccount(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := manager.ListAccounts(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("ListAccounts() with cancelled context = %v", err)
	}
}

func TestResolveLaunchAccountUsesSelectedOfflineAccount(t *testing.T) {
	ctx := context.Background()
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}

	account, err := manager.AuthenticateAccount(ctx, models.AccountAuthRequest{
		Type: models.AccountOffline, Username: "Selected",
	})
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveLaunchAccount(ctx, manager, models.Account{})
	if err != nil || resolved.ID != account.ID || resolved.Type != models.AccountOffline {
		t.Fatalf("ResolveLaunchAccount() = %+v, %v", resolved, err)
	}
	if _, err := ResolveLaunchAccount(ctx, manager, models.Account{Type: models.AccountMicrosoft}); !errors.Is(err, core.ErrAccountOperationUnsupported) {
		t.Fatalf("unsupported explicit account type was silently replaced: %v", err)
	}
	if _, err := ResolveLaunchAccount(ctx, manager, models.Account{
		ID: account.ID, Type: models.AccountMicrosoft,
	}); err == nil {
		t.Fatal("account type mismatch was accepted")
	}
}

func TestManagerConcurrentAccountOperations(t *testing.T) {
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	const accountCount = 24
	var wait sync.WaitGroup
	errs := make(chan error, accountCount)
	for index := 0; index < accountCount; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			account, err := manager.AuthenticateAccount(context.Background(), models.AccountAuthRequest{
				Type: models.AccountOffline, Username: fmt.Sprintf("Player_%02d", index),
			})
			if err != nil {
				errs <- err
				return
			}
			if _, err := manager.GetAccount(context.Background(), account.ID); err != nil {
				errs <- err
			}
		}(index)
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	records, err := manager.ListAccounts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != accountCount {
		t.Fatalf("concurrent account count = %d, want %d", len(records), accountCount)
	}
}
