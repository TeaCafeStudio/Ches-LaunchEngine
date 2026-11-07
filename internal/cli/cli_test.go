package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	cleerrors "github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/errors"
)

func TestJSONResponsesAreSingleLineAndStructured(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStatus int
		wantCode   string
	}{
		{name: "version", args: []string{"--json", "-version"}},
		{name: "unsupported debug migration", args: []string{"debug", "--json"}, wantStatus: 1, wantCode: "CLI_ERROR"},
		{name: "invalid configuration", args: []string{"--json", "-config", t.TempDir() + "/missing.json"}, wantStatus: 1, wantCode: string(cleerrors.CodeInvalidConfiguration)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			status := Run(test.args, &stdout, &stderr)
			if status != test.wantStatus {
				t.Fatalf("Run() status = %d, want %d; stderr=%q", status, test.wantStatus, stderr.String())
			}
			if strings.Contains(stdout.String(), "\n") && strings.Count(stdout.String(), "\n") != 1 {
				t.Fatalf("JSON output contains more than one line: %q", stdout.String())
			}
			var response Response
			if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
				t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
			}
			if response.Success != (test.wantStatus == 0) {
				t.Fatalf("success=%t for status %d", response.Success, test.wantStatus)
			}
			if test.wantCode != "" {
				if response.Error == nil || response.Error.Code != test.wantCode {
					t.Fatalf("error response = %+v, want code %s", response.Error, test.wantCode)
				}
			} else if response.Error != nil {
				t.Fatalf("unexpected error: %+v", response.Error)
			}
			if stderr.Len() != 0 {
				t.Fatalf("JSON mode wrote diagnostics to stderr: %q", stderr.String())
			}
		})
	}
}

func TestTextResponsesKeepHumanReadableOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := Run([]string{"-version"}, &stdout, &stderr); status != 0 {
		t.Fatalf("Run() = %d, stderr=%q", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ChesLaunchEngine") || strings.Contains(stdout.String(), `"success"`) {
		t.Fatalf("unexpected text output: %q", stdout.String())
	}
}

func TestJSONErrorRedactsSensitiveValues(t *testing.T) {
	encoded, err := json.Marshal(Response{
		Success: false,
		Error:   toCLIError(cleerrors.New(cleerrors.CodeAuthenticationFailed, "accessToken=secret-value")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "secret-value") || !strings.Contains(string(encoded), "[REDACTED]") {
		t.Fatalf("JSON error leaked a sensitive value: %s", encoded)
	}
}
