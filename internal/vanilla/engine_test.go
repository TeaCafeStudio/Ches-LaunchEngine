package vanilla

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/config"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/accounts"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/core"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

func TestValidateAcceptsOfflineVanillaRequest(t *testing.T) {
	engine, err := NewEngine(EngineOptions{
		Config: config.Default(),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	request := models.LaunchRequest{
		VersionID: "1.21.1", GameDirectory: t.TempDir(),
		Account: models.Account{Name: "Offline_User", Type: models.AccountOffline},
		Loader:  models.Loader{Type: models.LoaderVanilla},
	}
	if err := engine.Validate(context.Background(), request); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsUnsupportedFeatures(t *testing.T) {
	engine, err := NewEngine(EngineOptions{Config: config.Default()})
	if err != nil {
		t.Fatal(err)
	}
	base := models.LaunchRequest{
		VersionID: "1.21.1", GameDirectory: t.TempDir(),
		Account: models.Account{Name: "Player"},
	}
	tests := []struct {
		name   string
		change func(*models.LaunchRequest)
	}{
		{name: "Microsoft account", change: func(request *models.LaunchRequest) {
			request.Account.Type = models.AccountMicrosoft
		}},
		{name: "Fabric loader", change: func(request *models.LaunchRequest) {
			request.Loader.Type = models.LoaderFabric
		}},
		{name: "invalid username", change: func(request *models.LaunchRequest) {
			request.Account.Name = "bad name"
		}},
		{name: "invalid custom UUID", change: func(request *models.LaunchRequest) {
			request.Account.UUID = "not-a-uuid"
		}},
		{name: "invalid profile UUID", change: func(request *models.LaunchRequest) {
			request.Account.Profile = models.GameProfile{ID: "not-a-uuid", Name: "Player"}
			request.Account.UUID = ""
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := base
			test.change(&request)
			if err := engine.Validate(context.Background(), request); err == nil {
				t.Fatal("Validate() succeeded for unsupported request")
			}
		})
	}
}

func TestSubscribeTaskReportsStatusChanges(t *testing.T) {
	engine, err := NewEngine(EngineOptions{
		Config: config.Default(),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	engine.tasks["task-1"] = &taskState{
		task: models.LaunchTask{ID: "task-1", Status: models.LaunchResolving},
	}
	events, unsubscribe, err := engine.SubscribeTask("task-1")
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()
	initial := <-events
	if initial.Status != models.LaunchResolving {
		t.Fatalf("initial event status = %s", initial.Status)
	}
	engine.updateTask("task-1", models.LaunchValidating, nil)
	next := <-events
	if next.Status != models.LaunchValidating {
		t.Fatalf("next event status = %s", next.Status)
	}
}

type testYggdrasilProvider struct {
	resolveErr error
}

func (provider *testYggdrasilProvider) Type() models.AccountType {
	return models.AccountYggdrasil
}

func (provider *testYggdrasilProvider) Authenticate(_ context.Context, _ models.AccountAuthRequest) (models.Account, error) {
	return models.Account{
		ID: "test-account", Type: models.AccountYggdrasil, Name: "Ygg_Player",
		UUID:       "0123456789abcdef0123456789abcdef",
		AuthServer: "https://auth.example/api/yggdrasil",
		Profile: models.GameProfile{
			ID: "0123456789abcdef0123456789abcdef", Name: "Ygg_Player",
			Properties: []models.ProfileProperty{{Name: "textures", Value: "signed-value", Signature: "signed-signature"}},
		},
	}, nil
}

func (provider *testYggdrasilProvider) Refresh(context.Context, models.Account) (models.Account, error) {
	return models.Account{}, core.ErrAccountOperationUnsupported
}

func (provider *testYggdrasilProvider) Validate(context.Context, models.Account) error {
	return nil
}

func (provider *testYggdrasilProvider) GetProfile(_ context.Context, account models.Account) (models.GameProfile, error) {
	return account.Profile, nil
}

func (provider *testYggdrasilProvider) Logout(context.Context, models.Account) error {
	return core.ErrAccountOperationUnsupported
}

func (provider *testYggdrasilProvider) ResolveLaunchAccount(_ context.Context, account models.Account) (models.Account, error) {
	if provider.resolveErr != nil {
		return models.Account{}, provider.resolveErr
	}
	account.Session = models.AccountSession{AccessToken: "test-access-token", ClientToken: "test-client-token"}
	return account, nil
}

func TestValidateResolvesStoredYggdrasilAccountThroughCoreManager(t *testing.T) {
	provider := &testYggdrasilProvider{}
	manager, err := accounts.NewManager("", provider)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := manager.AuthenticateAccount(context.Background(), models.AccountAuthRequest{
		Type: models.AccountYggdrasil,
	})
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(EngineOptions{
		Config: config.Default(), Accounts: manager,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	request := models.LaunchRequest{
		VersionID: "1.21.1", GameDirectory: t.TempDir(),
		Account: models.Account{ID: stored.ID, Type: models.AccountYggdrasil},
		Loader:  models.Loader{Type: models.LoaderVanilla},
	}
	if err := engine.Validate(context.Background(), request); err != nil {
		t.Fatalf("Validate() with a provider-resolved Yggdrasil account: %v", err)
	}
}

func TestYggdrasilAuthenticationFailureDoesNotCreateProcess(t *testing.T) {
	provider := &testYggdrasilProvider{resolveErr: errors.New("validation failed")}
	manager, err := accounts.NewManager("", provider)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := manager.AuthenticateAccount(context.Background(), models.AccountAuthRequest{
		Type: models.AccountYggdrasil,
	})
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(EngineOptions{
		Config: config.Default(), Accounts: manager,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	task, launchErr := engine.Launch(context.Background(), models.LaunchRequest{
		VersionID: "1.21.1", GameDirectory: t.TempDir(),
		Account: models.Account{ID: stored.ID},
	})
	if launchErr == nil {
		t.Fatal("Launch() succeeded despite an authentication failure")
	}
	if task.Status != models.LaunchFailed || task.ProcessID != 0 {
		t.Fatalf("failed authentication created a process or wrong task state: %+v", task)
	}
}

func TestAddAuthlibInjectorUsesJVMArgumentPosition(t *testing.T) {
	args := []string{"-Xmx2G", "net.minecraft.client.main.Main", "--username", "Player"}
	result, err := addAuthlibInjector(args, "net.minecraft.client.main.Main", `C:\game\agent.jar`, "https://auth.example/api/yggdrasil")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"-Xmx2G", "-javaagent:C:\\game\\agent.jar=https://auth.example/api/yggdrasil",
		"-Dauthlibinjector.noLogFile", "net.minecraft.client.main.Main", "--username", "Player",
	}
	if len(result) != len(want) {
		t.Fatalf("argument count = %d, want %d: %#v", len(result), len(want), result)
	}
	for index := range want {
		if result[index] != want[index] {
			t.Fatalf("argument %d = %q, want %q", index, result[index], want[index])
		}
	}
}
