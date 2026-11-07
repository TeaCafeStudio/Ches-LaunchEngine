package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	cleerrors "github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/errors"
)

func TestDecodeUsesDefaults(t *testing.T) {
	cfg, err := Decode(strings.NewReader(`{"version":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Download.Concurrency != 8 {
		t.Fatalf("download concurrency = %d, want 8", cfg.Download.Concurrency)
	}
	if cfg.Logging.Level != "INFO" {
		t.Fatalf("logging level = %q, want INFO", cfg.Logging.Level)
	}
	if cfg.Launch.ArgumentMode != "auto" || !cfg.DebugTerminal.Enabled || cfg.DebugTerminal.HistoryLimit != 1000 {
		t.Fatalf("launch/debug-terminal defaults were not applied: %+v %+v", cfg.Launch, cfg.DebugTerminal)
	}
}

func TestDecodeAccountProfileStorePath(t *testing.T) {
	cfg, err := Decode(strings.NewReader(`{"version":1,"account":{"storePath":"C:\\Users\\Player\\accounts.json"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Account.StorePath != `C:\Users\Player\accounts.json` {
		t.Fatalf("account store path = %q", cfg.Account.StorePath)
	}
}

func TestDecodeAcceptsOperatingSystemCredentialStore(t *testing.T) {
	if _, err := Decode(strings.NewReader(`{"version":1,"account":{"credentialStore":"os"}}`)); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeRejectsUnsupportedOnlineAccountSettings(t *testing.T) {
	for _, value := range []string{
		`{"version":1,"account":{"credentialStore":"keychain"}}`,
	} {
		if _, err := Decode(strings.NewReader(value)); err == nil {
			t.Fatalf("Decode(%s) accepted unsupported account settings", value)
		}
	}
}

func TestDecodeAcceptsMicrosoftDefaultProvider(t *testing.T) {
	cfg, err := Decode(strings.NewReader(`{"version":1,"account":{"defaultProvider":"microsoft","microsoft":{"clientId":"client-id"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Account.DefaultProvider != "microsoft" || cfg.Account.Microsoft.ClientID != "client-id" {
		t.Fatalf("account settings = %+v", cfg.Account)
	}
}

func TestLoadLaunchRequestStrictlyDecodesJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "launch.json")
	if err := os.WriteFile(path, []byte(`{"versionId":"1.21.1","gameDirectory":".minecraft"}`), 0600); err != nil {
		t.Fatal(err)
	}
	request, err := LoadLaunchRequest(path)
	if err != nil || request.VersionID != "1.21.1" {
		t.Fatalf("LoadLaunchRequest() = %+v, %v", request, err)
	}
	if err := os.WriteFile(path, []byte(`{"versionId":"1.21.1","unknown":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLaunchRequest(path); err == nil {
		t.Fatal("LoadLaunchRequest accepted an unknown field")
	}
}

func TestDecodeRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "missing version", data: `{}`},
		{name: "unknown field", data: `{"version":1,"unexpected":true}`},
		{name: "invalid concurrency", data: `{"version":1,"download":{"concurrency":0}}`},
		{name: "trailing value", data: `{"version":1} {}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Decode(strings.NewReader(test.data))
			if err == nil {
				t.Fatal("Decode() succeeded, want error")
			}
			if engineErr, ok := err.(*cleerrors.Error); !ok || engineErr.Code != cleerrors.CodeInvalidConfiguration {
				t.Fatalf("Decode() error = %T %v, want INVALID_CONFIGURATION", err, err)
			}
		})
	}
}
