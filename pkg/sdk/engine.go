package sdk

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/config"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/vanilla"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/core"
)

type Options struct {
	GameDirectory       string
	VersionsDirectory   string
	JavaPath            string
	Offline             bool
	DownloadConcurrency int
	DownloadRetries     int
	DownloadTimeout     time.Duration
	LaunchTimeout       time.Duration
	Logger              *slog.Logger
	HTTPClient          *http.Client
	ManifestURL         string
}

func NewEngine(options Options) (core.MinecraftEngine, error) {
	return newEngine(options)
}

func NewEngineWithConfig(cfg config.Config, logger *slog.Logger, httpClient *http.Client, manifestURL string) (core.MinecraftEngine, error) {
	return newConfiguredEngine(cfg, logger, httpClient, manifestURL)
}

func NewDebugEngine(options Options) (core.DebugEngine, error) {
	engine, err := newEngine(options)
	if err != nil {
		return nil, err
	}
	debugEngine, ok := engine.(core.DebugEngine)
	if !ok {
		closeErr := engine.Close()
		return nil, errors.Join(fmt.Errorf("engine does not implement the Debug Core API"), closeErr)
	}
	return debugEngine, nil
}

func NewDebugEngineWithConfig(cfg config.Config, logger *slog.Logger, httpClient *http.Client, manifestURL string) (core.DebugEngine, error) {
	engine, err := newConfiguredEngine(cfg, logger, httpClient, manifestURL)
	if err != nil {
		return nil, err
	}
	debugEngine, ok := engine.(core.DebugEngine)
	if !ok {
		closeErr := engine.Close()
		return nil, errors.Join(fmt.Errorf("engine does not implement the Debug Core API"), closeErr)
	}
	return debugEngine, nil
}

func newEngine(options Options) (core.MinecraftEngine, error) {
	cfg := config.Default()
	cfg.Paths.GameDirectory = options.GameDirectory
	cfg.Paths.VersionsDirectory = options.VersionsDirectory
	cfg.Java.Path = options.JavaPath
	cfg.Download.Offline = options.Offline
	if options.DownloadConcurrency > 0 {
		cfg.Download.Concurrency = options.DownloadConcurrency
	}
	if options.DownloadRetries > 0 {
		cfg.Download.Retries = options.DownloadRetries
	}
	if options.DownloadTimeout > 0 {
		cfg.Download.TimeoutSec = durationSeconds(options.DownloadTimeout)
	}
	if options.LaunchTimeout > 0 {
		cfg.Launch.TimeoutSec = durationSeconds(options.LaunchTimeout)
	}
	return newConfiguredEngine(cfg, options.Logger, options.HTTPClient, options.ManifestURL)
}

func newConfiguredEngine(cfg config.Config, logger *slog.Logger, httpClient *http.Client, manifestURL string) (core.MinecraftEngine, error) {
	engine, err := vanilla.NewEngine(vanilla.EngineOptions{
		Config: cfg, Logger: logger,
		HTTPClient: httpClient, ManifestURL: manifestURL,
	})
	if err != nil {
		return nil, fmt.Errorf("create CLE engine: %w", err)
	}
	return engine, nil
}

func durationSeconds(duration time.Duration) int {
	seconds := int(duration / time.Second)
	if duration%time.Second != 0 {
		seconds++
	}
	if seconds < 1 {
		return 1
	}
	return seconds
}
