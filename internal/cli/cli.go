package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/config"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/logging"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/accounts"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/core"
	cleerrors "github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/errors"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/sdk"
)

type Response struct {
	Success bool      `json:"success"`
	Data    any       `json:"data"`
	Error   *CLIError `json:"error"`
}

type CLIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Result struct {
	Command string `json:"command"`
	Message string `json:"message,omitempty"`
	Version string `json:"version,omitempty"`
	Task    any    `json:"task,omitempty"`
}

func Run(args []string, stdout, stderr io.Writer) int {
	jsonOutput := containsJSONFlag(args)
	if len(args) > 0 && args[0] == "debug" {
		return report(stdout, stderr, jsonOutput, nil, errors.New("the Debug Terminal has moved to the separate cle-debug program; run cle-debug"))
	}
	flags := flag.NewFlagSet("cle", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: cle [--json] [-version] [-check-config file] [-config file] [-launch request.json]")
	}
	showVersion := flags.Bool("version", false, "print the CLE version")
	checkConfig := flags.String("check-config", "", "validate a JSON configuration file")
	configPath := flags.String("config", "", "load the CLE JSON configuration file")
	launchPath := flags.String("launch", "", "launch Minecraft with a JSON request file")
	flags.BoolVar(&jsonOutput, "json", jsonOutput, "write one JSON response to stdout")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return report(stdout, stderr, jsonOutput, nil, err)
	}
	if flags.NArg() != 0 {
		return report(stdout, stderr, jsonOutput, nil, fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " ")))
	}

	if *showVersion {
		return report(stdout, stderr, jsonOutput, Result{Command: "version", Version: Version}, nil)
	}
	cfg := config.Default()
	if *configPath != "" {
		loaded, err := config.LoadFile(*configPath)
		if err != nil {
			return report(stdout, stderr, jsonOutput, nil, err)
		}
		cfg = loaded
	}
	if *checkConfig != "" {
		if _, err := config.LoadFile(*checkConfig); err != nil {
			return report(stdout, stderr, jsonOutput, nil, err)
		}
		return report(stdout, stderr, jsonOutput, Result{Command: "check-config", Message: "configuration is valid"}, nil)
	}
	if *launchPath != "" {
		result, err := launch(cfg, *launchPath, jsonOutput)
		return report(stdout, stderr, jsonOutput, result, err)
	}
	if !jsonOutput {
		flags.Usage()
		return 0
	}
	return report(stdout, stderr, true, Result{Command: "help", Message: "No command selected."}, nil)
}

func launch(cfg config.Config, requestPath string, jsonOutput bool) (Result, error) {
	request, err := config.LoadLaunchRequest(requestPath)
	if err != nil {
		return Result{}, err
	}
	account := request.Account
	needsAccountStore := account.ID != "" ||
		(account.Name == "" && account.DisplayName == "" && account.Profile.Name == "" &&
			(account.Type == "" || account.Type == models.AccountOffline))
	var accountManager core.AccountManager
	if needsAccountStore {
		storePath := cfg.Account.StorePath
		if storePath == "" {
			storePath, err = accounts.DefaultStorePath()
			if err != nil {
				return Result{}, err
			}
		}
		accountManager, err = accounts.NewManager(storePath)
		if err != nil {
			return Result{}, fmt.Errorf("initialize account manager: %w", err)
		}
	}
	request.Account, err = accounts.ResolveLaunchAccount(context.Background(), accountManager, request.Account)
	if err != nil {
		return Result{}, fmt.Errorf("resolve launch account: %w", err)
	}

	logger, closeLogger, err := newLogger(cfg, jsonOutput)
	if err != nil {
		return Result{}, err
	}
	defer func() {
		if closeErr := closeLogger(); closeErr != nil {
			logger.Error("close logger", "error", core.RedactText(closeErr.Error()))
		}
	}()
	engine, err := sdk.NewDebugEngineWithConfig(cfg, logger, nil, "")
	if err != nil {
		return Result{}, err
	}
	defer func() {
		if closeErr := engine.Close(); closeErr != nil {
			logger.Warn("close engine", "error", core.RedactText(closeErr.Error()))
		}
	}()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	task, err := engine.Launch(ctx, request)
	if err != nil {
		return Result{}, err
	}
	result := Result{Command: "launch", Task: task}
	if !jsonOutput {
		fmt.Printf("Minecraft started: task=%s pid=%d\n", task.ID, task.ProcessID)
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if task.ProcessID != 0 {
				if err := engine.StopProcess(context.Background(), task.ProcessID, false); err != nil {
					return Result{}, err
				}
			} else if err := engine.CancelTask(context.Background(), task.ID); err != nil {
				return Result{}, err
			}
			return Result{}, ctx.Err()
		case <-ticker.C:
			task, err = engine.GetLaunchStatus(task.ID)
			if err != nil {
				return Result{}, err
			}
			result.Task = task
			switch task.Status {
			case models.LaunchExited:
				if task.ExitCode != nil && *task.ExitCode != 0 {
					return Result{}, fmt.Errorf("Minecraft exited with code %d", *task.ExitCode)
				}
				return result, nil
			case models.LaunchFailed:
				if task.Error != "" {
					return Result{}, errors.New(strings.TrimSpace(task.Error))
				}
				return Result{}, errors.New("Minecraft process failed")
			case models.LaunchCancelled:
				return Result{}, errors.New("Minecraft launch was cancelled")
			}
		}
	}
}

func newLogger(cfg config.Config, jsonOutput bool) (*slog.Logger, func() error, error) {
	if jsonOutput && cfg.Logging.Directory == "" {
		return slog.New(slog.NewTextHandler(io.Discard, nil)), func() error { return nil }, nil
	}
	console := cfg.Logging.Console
	if jsonOutput {
		console = false
	}
	return logging.New(logging.Options{
		Level: cfg.Logging.Level, Console: console,
		LogDir: cfg.Logging.Directory, InstanceID: cfg.Logging.InstanceID,
	})
}

func report(stdout, stderr io.Writer, jsonOutput bool, data any, err error) int {
	if jsonOutput {
		response := Response{Success: err == nil, Data: data}
		if err != nil {
			response.Error = toCLIError(err)
		}
		if encodeErr := json.NewEncoder(stdout).Encode(response); encodeErr != nil {
			fmt.Fprintln(stderr, core.RedactText(encodeErr.Error()))
			return 1
		}
		if err != nil {
			return 1
		}
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, core.RedactText(err.Error()))
		return 1
	}
	if result, ok := data.(Result); ok {
		switch result.Command {
		case "version":
			fmt.Fprintf(stdout, "ChesLaunchEngine %s\n", result.Version)
		case "check-config":
			fmt.Fprintln(stdout, result.Message)
		}
	}
	return 0
}

func toCLIError(err error) *CLIError {
	var engineError *cleerrors.Error
	if errors.As(err, &engineError) {
		return &CLIError{Code: string(engineError.Code), Message: core.RedactText(engineError.Message)}
	}
	return &CLIError{Code: "CLI_ERROR", Message: core.RedactText(err.Error())}
}

func containsJSONFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--json" || arg == "-json" {
			return true
		}
	}
	return false
}

const Version = "dev"
