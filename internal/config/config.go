package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	cleerrors "github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/errors"
)

const CurrentVersion = 1

type Config struct {
	Version       int           `json:"version"`
	Paths         Paths         `json:"paths"`
	Launch        Launch        `json:"launch"`
	Download      Download      `json:"download"`
	Java          Java          `json:"java"`
	Account       Account       `json:"account"`
	Logging       Logging       `json:"logging"`
	API           API           `json:"api"`
	DebugTerminal DebugTerminal `json:"debugTerminal"`
}

type Paths struct {
	GameDirectory     string `json:"gameDirectory,omitempty"`
	VersionsDirectory string `json:"versionsDirectory,omitempty"`
}

type Launch struct {
	ArgumentMode string `json:"argumentMode"`
	TimeoutSec   int    `json:"timeoutSeconds"`
}

type Download struct {
	Concurrency int  `json:"concurrency"`
	Retries     int  `json:"retries"`
	TimeoutSec  int  `json:"timeoutSeconds"`
	Offline     bool `json:"offline"`
}

type Java struct {
	Path string `json:"path,omitempty"`
}

type Account struct {
	DefaultProvider string `json:"defaultProvider,omitempty"`
	CredentialStore string `json:"credentialStore,omitempty"`
	StorePath       string `json:"storePath,omitempty"`
}

type Logging struct {
	Level      string `json:"level"`
	Console    bool   `json:"console"`
	Directory  string `json:"directory,omitempty"`
	InstanceID string `json:"instanceId,omitempty"`
}

type API struct {
	Enabled bool   `json:"enabled"`
	Address string `json:"address,omitempty"`
}

type DebugTerminal struct {
	Enabled      bool `json:"enabled"`
	HistoryLimit int  `json:"historyLimit"`
}

func Default() Config {
	return Config{
		Version: CurrentVersion,
		Launch: Launch{
			ArgumentMode: "auto",
			TimeoutSec:   3600,
		},
		Download: Download{
			Concurrency: 8,
			Retries:     3,
			TimeoutSec:  300,
		},
		Logging: Logging{
			Level:   "INFO",
			Console: true,
		},
		API: API{
			Address: "127.0.0.1:8765",
		},
		DebugTerminal: DebugTerminal{
			Enabled:      true,
			HistoryLimit: 1000,
		},
	}
}

func LoadFile(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, cleerrors.Wrap(cleerrors.CodeInvalidConfiguration, "open configuration file", err)
	}
	defer file.Close()

	return Decode(file)
}

func Decode(reader io.Reader) (Config, error) {
	cfg := Default()
	cfg.Version = 0

	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, cleerrors.Wrap(cleerrors.CodeInvalidConfiguration, "decode configuration JSON", err)
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values are not allowed")
		}
		return Config{}, cleerrors.Wrap(cleerrors.CodeInvalidConfiguration, "configuration must contain one JSON object", err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (cfg Config) Validate() error {
	if cfg.Version != CurrentVersion {
		return cleerrors.New(cleerrors.CodeInvalidConfiguration, fmt.Sprintf("unsupported or missing configuration version %d", cfg.Version))
	}
	switch strings.ToLower(cfg.Launch.ArgumentMode) {
	case "auto", "legacy", "modern":
	default:
		return cleerrors.New(cleerrors.CodeInvalidConfiguration, "launch.argumentMode must be auto, legacy, or modern")
	}
	if cfg.Launch.TimeoutSec < 1 {
		return cleerrors.New(cleerrors.CodeInvalidConfiguration, "launch.timeoutSeconds must be positive")
	}
	if cfg.Download.Concurrency < 1 || cfg.Download.Concurrency > 64 {
		return cleerrors.New(cleerrors.CodeInvalidConfiguration, "download.concurrency must be between 1 and 64")
	}
	if cfg.Download.Retries < 0 || cfg.Download.Retries > 10 {
		return cleerrors.New(cleerrors.CodeInvalidConfiguration, "download.retries must be between 0 and 10")
	}
	if cfg.Download.TimeoutSec < 1 {
		return cleerrors.New(cleerrors.CodeInvalidConfiguration, "download.timeoutSeconds must be positive")
	}

	switch strings.ToUpper(cfg.Logging.Level) {
	case "DEBUG", "INFO", "WARN", "ERROR":
	default:
		return cleerrors.New(cleerrors.CodeInvalidConfiguration, "logging.level must be DEBUG, INFO, WARN, or ERROR")
	}
	if cfg.Logging.Directory != "" && cfg.Logging.InstanceID == "" {
		return cleerrors.New(cleerrors.CodeInvalidConfiguration, "logging.instanceId is required when logging.directory is set")
	}
	if cfg.DebugTerminal.HistoryLimit < 0 || cfg.DebugTerminal.HistoryLimit > 10000 {
		return cleerrors.New(cleerrors.CodeInvalidConfiguration, "debugTerminal.historyLimit must be between 0 and 10000")
	}
	if cfg.Account.DefaultProvider != "" && cfg.Account.DefaultProvider != "offline" {
		return cleerrors.New(cleerrors.CodeInvalidConfiguration, "account.defaultProvider is not supported in Phase 4.1")
	}
	if strings.TrimSpace(cfg.Account.CredentialStore) != "" {
		return cleerrors.New(cleerrors.CodeInvalidConfiguration, "account.credentialStore is not supported in Phase 4.1")
	}
	if cfg.Account.StorePath != "" && strings.TrimSpace(cfg.Account.StorePath) == "" {
		return cleerrors.New(cleerrors.CodeInvalidConfiguration, "account.storePath cannot be blank")
	}
	if cfg.API.Enabled && strings.TrimSpace(cfg.API.Address) == "" {
		return cleerrors.New(cleerrors.CodeInvalidConfiguration, "api.address is required when the API is enabled")
	}
	return nil
}
