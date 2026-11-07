package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/config"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/logging"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/terminal"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/accounts"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/core"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/sdk"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, core.RedactText(err.Error()))
		os.Exit(1)
	}
}

func run(args []string, input io.Reader, output, errorOutput io.Writer) error {
	flags := flag.NewFlagSet("cle-debug", flag.ContinueOnError)
	flags.SetOutput(errorOutput)
	configPath := flags.String("config", "", "load CLE configuration or launch-request JSON")
	launchConfigPath := flags.String("launch-config", "", "default Minecraft launch request JSON")
	command := flags.String("command", "", "execute one Debug Terminal command and exit")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if *command == "" && input == nil {
		return errors.New("interactive input is unavailable; use --command")
	}
	cfg := config.Default()
	defaultLaunchConfig := *launchConfigPath
	if *configPath != "" {
		loaded, configErr := config.LoadFile(*configPath)
		if configErr == nil {
			cfg = loaded
		} else if defaultLaunchConfig == "" {
			if _, requestErr := config.LoadLaunchRequest(*configPath); requestErr != nil {
				return fmt.Errorf("load debug configuration or launch request: %w", configErr)
			}
			defaultLaunchConfig = *configPath
		} else {
			return configErr
		}
	}
	if !cfg.DebugTerminal.Enabled {
		return errors.New("Debug Terminal is disabled by debugTerminal.enabled")
	}
	if *command != "" {
		input = strings.NewReader(*command + "\n")
	}
	accountStorePath := cfg.Account.StorePath
	if accountStorePath == "" {
		var err error
		accountStorePath, err = accounts.DefaultStorePath()
		if err != nil {
			return err
		}
	}
	accountManager, err := accounts.NewManagerWithClientID(accountStorePath, cfg.Account.Microsoft.ClientID)
	if err != nil {
		return fmt.Errorf("initialize account manager: %w", err)
	}
	logger, closeLogger, err := newLogger(cfg)
	if err != nil {
		return fmt.Errorf("initialize logger: %w", err)
	}
	defer func() {
		if closeErr := closeLogger(); closeErr != nil {
			logger.Error("close logger", "error", core.RedactText(closeErr.Error()))
		}
	}()
	engine, err := sdk.NewDebugEngineWithConfigAndAccounts(cfg, logger, nil, "", accountManager)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := engine.Close(); closeErr != nil {
			logger.Warn("close engine", "error", core.RedactText(closeErr.Error()))
		}
	}()
	debugTerminal, err := terminal.New(terminal.Options{
		Engine: engine, Accounts: accountManager, Config: cfg, Version: version,
		DefaultLaunchConfig: defaultLaunchConfig,
		Input:               input, Output: output,
	})
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	runErr := debugTerminal.Run(ctx)
	if errors.Is(runErr, context.Canceled) {
		return nil
	}
	return runErr
}

func newLogger(cfg config.Config) (*slog.Logger, func() error, error) {
	if cfg.Logging.Directory == "" {
		return slog.New(slog.NewTextHandler(io.Discard, nil)), func() error { return nil }, nil
	}
	return logging.New(logging.Options{
		Level: cfg.Logging.Level, Console: false,
		LogDir: cfg.Logging.Directory, InstanceID: cfg.Logging.InstanceID,
	})
}
