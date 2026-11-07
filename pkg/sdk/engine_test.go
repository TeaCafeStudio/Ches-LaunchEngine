package sdk

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/config"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

func TestNewEngineReturnsUsableCore(t *testing.T) {
	engine, err := NewEngine(Options{
		GameDirectory: t.TempDir(), DownloadConcurrency: 2,
		DownloadTimeout: 1500 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Validate(context.Background(), launchRequest()); err != nil {
		t.Fatal(err)
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewDebugEngineExposesDebugCore(t *testing.T) {
	engine, err := NewDebugEngine(Options{GameDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if tasks := engine.ListLaunchTasks(); len(tasks) != 0 {
		t.Fatalf("new engine has unexpected launch tasks: %+v", tasks)
	}
	events, unsubscribe, err := engine.SubscribeEvents(1)
	if err != nil {
		t.Fatal(err)
	}
	unsubscribe()
	if _, open := <-events; open {
		t.Fatal("event subscription remained open after unsubscribe")
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestConfiguredFactoriesShareTheCoreEngine(t *testing.T) {
	cfg := config.Default()
	cfg.Paths.GameDirectory = t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine, err := NewEngineWithConfig(cfg, logger, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Validate(context.Background(), launchRequest()); err != nil {
		t.Fatal(err)
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	debugEngine, err := NewDebugEngineWithConfig(cfg, logger, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(debugEngine.ListLaunchTasks()) != 0 {
		t.Fatal("configured debug engine unexpectedly contains launch tasks")
	}
	if err := debugEngine.Close(); err != nil {
		t.Fatal(err)
	}
}

func launchRequest() models.LaunchRequest {
	return models.LaunchRequest{
		VersionID: "1.21.1",
		Account:   models.Account{Name: "SDKUser", Type: models.AccountOffline},
	}
}
