package vanilla

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/config"
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
