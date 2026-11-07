package logging

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRedactingHandlerMasksSensitiveAttributes(t *testing.T) {
	var output bytes.Buffer
	handler := &redactingHandler{
		next: slog.NewJSONHandler(&output, nil),
	}
	logger := slog.New(handler)
	logger.LogAttrs(context.Background(), slog.LevelInfo, "account",
		slog.String("accessToken", "do-not-log"),
		slog.Group("profile",
			slog.String("password", "also-secret"),
			slog.String("name", "Player"),
		),
	)

	got := output.String()
	if strings.Contains(got, "do-not-log") || strings.Contains(got, "also-secret") {
		t.Fatalf("log output contains sensitive value: %s", got)
	}
	if !strings.Contains(got, "[REDACTED]") || !strings.Contains(got, "Player") {
		t.Fatalf("log output did not preserve redacted and non-sensitive fields: %s", got)
	}
}

func TestRedactingHandlerWithAttrsMasksSensitiveAttributes(t *testing.T) {
	var output bytes.Buffer
	handler := (&redactingHandler{next: slog.NewJSONHandler(&output, nil)}).
		WithAttrs([]slog.Attr{slog.String("apiToken", "do-not-log")})

	if err := handler.Handle(context.Background(), slog.NewRecord(
		time.Time{}, slog.LevelInfo, "event", 0,
	)); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "do-not-log") {
		t.Fatalf("log output contains sensitive value: %s", output.String())
	}
}

func TestNewRequiresSafeInstanceForFileLogs(t *testing.T) {
	_, _, err := New(Options{LogDir: t.TempDir(), InstanceID: ".."})
	if err == nil {
		t.Fatal("New() succeeded with an unsafe instance ID")
	}
}

func TestNewWritesPerInstanceLogFile(t *testing.T) {
	dir := t.TempDir()
	logger, closeLogger, err := New(Options{
		Level:      "INFO",
		LogDir:     dir,
		InstanceID: "instance-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	logger.Info("started", "accessToken", "do-not-log")
	if err := closeLogger(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "instance-a.log"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "do-not-log") {
		t.Fatalf("log file contains sensitive value: %s", data)
	}
	if _, err := os.Stat(filepath.Join(dir, "instance-b.log")); !os.IsNotExist(err) {
		t.Fatalf("unexpected second instance log file, stat error: %v", err)
	}
}
