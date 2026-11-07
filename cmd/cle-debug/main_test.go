package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandModeRunsOneTerminalCommand(t *testing.T) {
	var output, errorOutput bytes.Buffer
	if err := run([]string{"--command", "version"}, strings.NewReader(""), &output, &errorOutput); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "ChesLaunchEngine dev") {
		t.Fatalf("command output = %q", output.String())
	}
	if errorOutput.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", errorOutput.String())
	}
}

func TestLaunchRequestCanBeTerminalDefault(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "cle.json")
	requestPath := filepath.Join(directory, "launch.json")
	configJSON := `{"version":1,"account":{"storePath":"` + strings.ReplaceAll(filepath.Join(directory, "accounts.json"), `\`, `\\`) + `"}}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(requestPath, []byte(`{"versionId":"1.21.1","gameDirectory":".minecraft","account":{"name":"Offline","type":"offline"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	var output, errorOutput bytes.Buffer
	if err := run([]string{"--config", configPath, "--launch-config", requestPath, "--command", "version"}, strings.NewReader(""), &output, &errorOutput); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "ChesLaunchEngine dev") {
		t.Fatalf("default launch request was not accepted: %q", output.String())
	}
}

func TestDebugTerminalRejectsInvalidCommandsAndConfiguration(t *testing.T) {
	var output, errorOutput bytes.Buffer
	if err := run([]string{"--command", "unknown-command"}, strings.NewReader(""), &output, &errorOutput); err != nil {
		t.Fatalf("terminal displays invalid command errors as output: %v", err)
	}
	if !strings.Contains(output.String(), "unknown command") {
		t.Fatalf("missing invalid-command diagnostic: %q", output.String())
	}
	output.Reset()
	if err := run([]string{"--config", filepath.Join(t.TempDir(), "missing.json"), "--command", "help"}, strings.NewReader(""), &output, &errorOutput); err == nil {
		t.Fatal("run accepted a missing configuration")
	}
}
