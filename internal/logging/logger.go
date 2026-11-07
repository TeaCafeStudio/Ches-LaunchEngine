package logging

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type Options struct {
	Level      string
	Console    bool
	LogDir     string
	InstanceID string
}

func New(options Options) (*slog.Logger, func() error, error) {
	level, err := parseLevel(options.Level)
	if err != nil {
		return nil, nil, err
	}

	var writers []io.Writer
	var files []*os.File
	if options.Console {
		writers = append(writers, os.Stderr)
	}

	if options.LogDir != "" {
		if !validInstanceID(options.InstanceID) {
			return nil, nil, errors.New("a safe instance ID is required for file logging")
		}
		if err := os.MkdirAll(options.LogDir, 0700); err != nil {
			return nil, nil, fmt.Errorf("create log directory: %w", err)
		}
		path := filepath.Join(options.LogDir, options.InstanceID+".log")
		file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return nil, nil, fmt.Errorf("open log file: %w", err)
		}
		files = append(files, file)
		writers = append(writers, file)
	}

	if len(writers) == 0 {
		return nil, nil, errors.New("logging requires console output or a log directory")
	}

	handler := slog.NewJSONHandler(io.MultiWriter(writers...), &slog.HandlerOptions{Level: level})
	logger := slog.New(&redactingHandler{next: handler})
	closeFiles := func() error {
		var closeErr error
		for _, file := range files {
			closeErr = errors.Join(closeErr, file.Close())
		}
		return closeErr
	}
	return logger, closeFiles, nil
}

func parseLevel(value string) (slog.Level, error) {
	switch strings.ToUpper(value) {
	case "", "INFO":
		return slog.LevelInfo, nil
	case "DEBUG":
		return slog.LevelDebug, nil
	case "WARN":
		return slog.LevelWarn, nil
	case "ERROR":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unsupported log level %q", value)
	}
}

func validInstanceID(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') &&
			(char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') &&
			char != '-' && char != '_' {
			return false
		}
	}
	return true
}

type redactingHandler struct {
	next slog.Handler
}

func (handler *redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return handler.next.Enabled(ctx, level)
}

func (handler *redactingHandler) Handle(ctx context.Context, record slog.Record) error {
	redacted := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	var attrs []slog.Attr
	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, redactAttr(attr))
		return true
	})
	redacted.AddAttrs(attrs...)
	return handler.next.Handle(ctx, redacted)
}

func (handler *redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for index, attr := range attrs {
		redacted[index] = redactAttr(attr)
	}
	return &redactingHandler{next: handler.next.WithAttrs(redacted)}
}

func (handler *redactingHandler) WithGroup(name string) slog.Handler {
	return &redactingHandler{next: handler.next.WithGroup(name)}
}

func redactAttr(attr slog.Attr) slog.Attr {
	key := strings.ToLower(attr.Key)
	for _, marker := range []string{
		"token", "secret", "password", "authorization", "credential",
		"api_key", "api-key", "apikey", "access_key", "access-key", "accesskey",
		"private_key", "private-key", "privatekey", "cookie", "bearer",
	} {
		if strings.Contains(key, marker) {
			return slog.String(attr.Key, "[REDACTED]")
		}
	}
	if attr.Value.Kind() == slog.KindGroup {
		group := attr.Value.Group()
		for index := range group {
			group[index] = redactAttr(group[index])
		}
		attr.Value = slog.GroupValue(group...)
	}
	return attr
}
