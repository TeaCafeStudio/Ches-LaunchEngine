package vanilla

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/arguments"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/authlibinjector"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/config"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/download"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/metadata"
	javaruntime "github.com/TeaCafeStudio/Ches-LaunchEngine/internal/runtime"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/accounts"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/core"
	cleerrors "github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/errors"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

type EngineOptions struct {
	Config      config.Config
	Logger      *slog.Logger
	HTTPClient  *http.Client
	ManifestURL string
	Accounts    core.AccountManager
}

type taskState struct {
	task      models.LaunchTask
	cancel    context.CancelFunc
	proc      *os.Process
	done      chan struct{}
	stopped   bool
	debug     core.LaunchDebugInfo
	downloads map[string]bool
}

type Engine struct {
	config      config.Config
	logger      *slog.Logger
	downloader  *download.Manager
	resolver    *metadata.Resolver
	accounts    core.AccountManager
	mu          sync.RWMutex
	tasks       map[string]*taskState
	subscribers map[string]map[uint64]chan models.LaunchStatusEvent
	eventSubs   map[uint64]chan core.Event
	eventLog    []core.Event
	nextID      atomic.Uint64
	nextSubID   atomic.Uint64
	nextEventID atomic.Uint64
	closed      bool
}

func NewEngine(options EngineOptions) (*Engine, error) {
	cfg := options.Config
	if cfg.Version == 0 {
		cfg = config.Default()
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	timeout := time.Duration(cfg.Download.TimeoutSec) * time.Second
	downloader, err := download.New(download.Options{
		Concurrency: cfg.Download.Concurrency,
		Retries:     cfg.Download.Retries,
		Timeout:     timeout,
		Client:      options.HTTPClient,
		Offline:     cfg.Download.Offline,
	})
	if err != nil {
		return nil, err
	}
	resolver, err := metadata.NewResolver(metadata.ResolverOptions{
		Downloader: downloader, HTTPClient: options.HTTPClient, ManifestURL: options.ManifestURL,
	})
	if err != nil {
		return nil, err
	}
	return &Engine{
		config: cfg, logger: options.Logger, downloader: downloader,
		resolver: resolver, accounts: options.Accounts, tasks: make(map[string]*taskState),
		subscribers: make(map[string]map[uint64]chan models.LaunchStatusEvent),
		eventSubs:   make(map[uint64]chan core.Event),
	}, nil
}

func (engine *Engine) Validate(ctx context.Context, request models.LaunchRequest) error {
	account, err := accounts.ResolveLaunchAccount(ctx, engine.accounts, request.Account)
	if err != nil {
		return err
	}
	request.Account = account
	return engine.validateResolved(request)
}

func (engine *Engine) validateResolved(request models.LaunchRequest) error {
	if strings.TrimSpace(request.VersionID) == "" {
		return cleerrors.New(cleerrors.CodeVersionJSONInvalid, "versionId is required")
	}
	if strings.TrimSpace(request.GameDirectory) == "" && strings.TrimSpace(engine.config.Paths.GameDirectory) == "" {
		return cleerrors.New(cleerrors.CodeInvalidGameDirectory, "gameDirectory is required")
	}
	username := request.Account.Name
	if username == "" {
		username = request.Account.DisplayName
	}
	if username == "" {
		username = request.Account.Profile.Name
	}
	uuid := request.Account.UUID
	if uuid == "" {
		uuid = request.Account.Profile.ID
	}
	switch request.Account.Type {
	case models.AccountOffline, "":
		if err := accounts.ValidateOfflineIdentity(username, uuid); err != nil {
			return cleerrors.Wrap(cleerrors.CodeAuthenticationFailed, "invalid offline account", err)
		}
	case models.AccountYggdrasil:
		if request.Account.ID == "" || request.Account.AuthServer == "" {
			return cleerrors.New(cleerrors.CodeAccountNotFound, "Yggdrasil launch requires a stored account")
		}
		if request.Account.Profile.ID == "" || request.Account.Profile.Name == "" {
			return cleerrors.New(cleerrors.CodeProfileUnavailable, "Yggdrasil account has no game profile")
		}
		if request.Account.Session.AccessToken == "" || request.Account.Session.ClientToken == "" {
			return cleerrors.New(cleerrors.CodeSessionInvalid, "Yggdrasil launch requires a validated session")
		}
		if username != request.Account.Profile.Name || uuid != request.Account.Profile.ID {
			return cleerrors.New(cleerrors.CodeProfileUnavailable, "Yggdrasil launch identity does not match the selected game profile")
		}
	case models.AccountMicrosoft:
		if request.Account.ID == "" {
			return cleerrors.New(cleerrors.CodeAccountNotFound, "Microsoft launch requires a stored account")
		}
		if request.Account.Profile.ID == "" || request.Account.Profile.Name == "" {
			return cleerrors.New(cleerrors.CodeProfileUnavailable, "Microsoft account has no game profile")
		}
		if request.Account.Session.AccessToken == "" {
			return cleerrors.New(cleerrors.CodeSessionInvalid, "Microsoft launch requires a validated Minecraft session")
		}
		if username != request.Account.Profile.Name || uuid != request.Account.Profile.ID {
			return cleerrors.New(cleerrors.CodeProfileUnavailable, "Microsoft launch identity does not match the selected game profile")
		}
	default:
		return cleerrors.New(cleerrors.CodeAccountTypeUnsupported, "this account type is not supported for Minecraft launch")
	}
	if request.Loader.Type != "" && request.Loader.Type != models.LoaderVanilla {
		return cleerrors.New(cleerrors.CodeLoaderInstallFailed, "only Vanilla versions are supported in Phase 2")
	}
	if request.ArgumentMode != "" && request.ArgumentMode != models.ArgumentModeAuto &&
		request.ArgumentMode != models.ArgumentModeLegacy && request.ArgumentMode != models.ArgumentModeModern {
		return cleerrors.New(cleerrors.CodeArgumentResolutionFailed, "argumentMode must be auto, legacy, or modern")
	}
	if request.DownloadOptions.Concurrency < 0 || request.DownloadOptions.Concurrency > 64 {
		return cleerrors.New(cleerrors.CodeInvalidConfiguration, "download concurrency must be between 1 and 64")
	}
	if request.DownloadOptions.Retries < 0 || request.DownloadOptions.Retries > 10 {
		return cleerrors.New(cleerrors.CodeInvalidConfiguration, "download retries must be between 0 and 10")
	}
	if request.WorkingDirectory != "" {
		info, err := os.Stat(request.WorkingDirectory)
		if err != nil || !info.IsDir() {
			return cleerrors.Wrap(cleerrors.CodeInvalidGameDirectory, "workingDirectory must refer to an existing directory", err)
		}
	}
	for key, value := range request.Environment {
		if key == "" || strings.ContainsAny(key, "=\x00") || strings.ContainsRune(value, '\x00') {
			return cleerrors.New(cleerrors.CodeInvalidConfiguration, "environment contains an invalid key or value")
		}
	}
	return nil
}

func (engine *Engine) Launch(ctx context.Context, request models.LaunchRequest) (models.LaunchTask, error) {
	taskID, operationCtx, cancel, _, err := engine.createLaunchTask(ctx, request)
	if err != nil {
		return models.LaunchTask{}, err
	}
	return engine.runLaunch(taskID, operationCtx, cancel, request)
}

func (engine *Engine) StartLaunch(ctx context.Context, request models.LaunchRequest) (models.LaunchTask, error) {
	taskID, operationCtx, cancel, task, err := engine.createLaunchTask(ctx, request)
	if err != nil {
		return models.LaunchTask{}, err
	}
	go func() {
		if _, launchErr := engine.runLaunch(taskID, operationCtx, cancel, request); launchErr != nil {
			engine.logger.Warn("asynchronous Minecraft launch failed", "taskId", taskID, "error", core.RedactText(launchErr.Error()))
		}
	}()
	return task, nil
}

func (engine *Engine) createLaunchTask(ctx context.Context, request models.LaunchRequest) (string, context.Context, context.CancelFunc, models.LaunchTask, error) {
	taskID := fmt.Sprintf("cle-%d-%d", time.Now().UnixNano(), engine.nextID.Add(1))
	accountType := request.Account.Type
	if accountType == "" && engine.accounts != nil {
		var account models.Account
		var err error
		if request.Account.ID != "" {
			account, err = engine.accounts.GetAccount(ctx, request.Account.ID)
		} else if request.Account.Name == "" && request.Account.DisplayName == "" && request.Account.Profile.Name == "" {
			account, err = engine.accounts.ActiveAccount(ctx)
		}
		if err == nil {
			accountType = account.Type
		}
	}
	hasInlineIdentity := request.Account.Name != "" || request.Account.DisplayName != "" ||
		request.Account.Profile.Name != ""
	if accountType == "" && request.Account.ID == "" && (engine.accounts == nil || hasInlineIdentity) {
		accountType = models.AccountOffline
	}
	loaderType := request.Loader.Type
	if loaderType == "" {
		loaderType = models.LoaderVanilla
	}
	gameDirectory, _ := engine.gameDirectory(request)
	task := models.LaunchTask{
		ID: taskID, Status: models.LaunchResolving, StartedAt: time.Now().UTC(),
		VersionID: request.VersionID, AccountType: accountType,
		LoaderType: loaderType, GameDirectory: gameDirectory,
		JavaPath: firstNonEmpty(request.JavaPath, engine.config.Java.Path),
	}
	operationCtx, cancel := context.WithTimeout(ctx, time.Duration(engine.config.Launch.TimeoutSec)*time.Second)
	engine.mu.Lock()
	if engine.closed {
		engine.mu.Unlock()
		cancel()
		return "", nil, nil, models.LaunchTask{}, errors.New("Minecraft engine is closed")
	}
	engine.tasks[taskID] = &taskState{task: task, cancel: cancel, debug: core.LaunchDebugInfo{
		Task: task, VersionID: request.VersionID, JavaPath: task.JavaPath,
		AccountType: accountType, GameDirectory: gameDirectory, LoaderType: loaderType,
		Stage: models.LaunchResolving,
	}}
	engine.publishStatusLocked(engine.tasks[taskID], "")
	engine.mu.Unlock()
	return taskID, operationCtx, cancel, task, nil
}

func (engine *Engine) runLaunch(taskID string, operationCtx context.Context, cancel context.CancelFunc, request models.LaunchRequest) (models.LaunchTask, error) {
	defer cancel()

	fail := func(err error) (models.LaunchTask, error) {
		if operationCtx.Err() != nil {
			engine.updateTask(taskID, models.LaunchCancelled, operationCtx.Err())
			current, _ := engine.GetLaunchStatus(taskID)
			return current, operationCtx.Err()
		}
		engine.updateTask(taskID, models.LaunchFailed, err)
		current, _ := engine.GetLaunchStatus(taskID)
		return current, err
	}
	account, err := accounts.ResolveLaunchAccount(operationCtx, engine.accounts, request.Account)
	if err != nil {
		return fail(err)
	}
	request.Account = account
	if err := engine.validateResolved(request); err != nil {
		return fail(err)
	}
	if request.Account.Type == "" {
		request.Account.Type = models.AccountOffline
	}
	if request.Loader.Type == "" {
		request.Loader.Type = models.LoaderVanilla
	}
	engine.mu.Lock()
	if state := engine.tasks[taskID]; state != nil {
		state.task.AccountType = request.Account.Type
		state.task.LoaderType = request.Loader.Type
		state.debug.AccountType = request.Account.Type
		state.debug.LoaderType = request.Loader.Type
	}
	engine.mu.Unlock()
	if request.ArgumentMode == "" {
		request.ArgumentMode = models.ArgumentMode(strings.ToLower(engine.config.Launch.ArgumentMode))
	}
	gameDirectory, err := engine.gameDirectory(request)
	if err != nil {
		return fail(err)
	}
	if err := os.MkdirAll(gameDirectory, 0755); err != nil {
		return fail(cleerrors.Wrap(cleerrors.CodeInvalidGameDirectory, "create game directory", err))
	}
	engine.mu.Lock()
	if state := engine.tasks[taskID]; state != nil {
		state.task.GameDirectory = gameDirectory
		state.debug.GameDirectory = gameDirectory
		state.debug.VersionID = request.VersionID
		state.debug.AccountType = request.Account.Type
		state.debug.LoaderType = request.Loader.Type
	}
	engine.mu.Unlock()
	var versionDirectory string
	switch {
	case request.VersionDirectory != "":
		versionDirectory, err = filepath.Abs(request.VersionDirectory)
		if err == nil {
			err = os.MkdirAll(versionDirectory, 0755)
		}
		if err == nil {
			versionDirectory, err = filepath.EvalSymlinks(versionDirectory)
		}
	case engine.config.Paths.VersionsDirectory != "":
		versionsDirectory, pathErr := filepath.Abs(engine.config.Paths.VersionsDirectory)
		if pathErr == nil {
			pathErr = os.MkdirAll(versionsDirectory, 0755)
		}
		if pathErr == nil {
			versionDirectory, pathErr = download.ResolveUnderRoot(versionsDirectory, request.VersionID)
		}
		err = pathErr
	default:
		versionDirectory, err = download.ResolveUnderRoot(gameDirectory, filepath.Join("versions", request.VersionID))
	}
	if err != nil {
		return fail(cleerrors.Wrap(cleerrors.CodeInvalidVersionDirectory, "resolve version directory", err))
	}
	if err := os.MkdirAll(versionDirectory, 0755); err != nil {
		return fail(cleerrors.Wrap(cleerrors.CodeInvalidVersionDirectory, "create version directory", err))
	}

	version, err := engine.resolver.ResolveAt(operationCtx, gameDirectory, versionDirectory, request.VersionID, request.DownloadOptions.Offline || engine.config.Download.Offline)
	if err != nil {
		return fail(err)
	}
	if operationCtx.Err() != nil {
		return fail(operationCtx.Err())
	}
	request.GameDirectory = gameDirectory

	engine.updateTask(taskID, models.LaunchValidating, nil)
	javaPath := request.JavaPath
	if javaPath == "" {
		javaPath = engine.config.Java.Path
	}
	java, err := javaruntime.DetectJava(operationCtx, javaPath)
	if err != nil {
		return fail(err)
	}
	engine.mu.Lock()
	if state := engine.tasks[taskID]; state != nil {
		state.task.JavaPath = java.Path
		state.debug.JavaPath = java.Path
	}
	engine.mu.Unlock()
	if version.JavaVersion != nil {
		if err := javaruntime.RequireMajor(java, version.JavaVersion.MajorVersion); err != nil {
			return fail(err)
		}
	}
	if err := javaruntime.RequireArchitecture(java, runtime.GOARCH); err != nil {
		return fail(err)
	}
	if operationCtx.Err() != nil {
		return fail(operationCtx.Err())
	}

	engine.updateTask(taskID, models.LaunchDownloading, nil)
	var authlibPath string
	if request.Account.Type == models.AccountYggdrasil {
		authlibPath, err = authlibinjector.Resolve(operationCtx, gameDirectory,
			request.DownloadOptions.Offline || engine.config.Download.Offline)
		if err != nil {
			return fail(err)
		}
	}
	concurrency := request.DownloadOptions.Concurrency
	if concurrency == 0 {
		concurrency = engine.config.Download.Concurrency
	}
	features := map[string]bool{
		"is_demo_user":               false,
		"has_custom_resolution":      request.Window.Width > 0 && request.Window.Height > 0,
		"has_quick_plays_support":    false,
		"is_quick_play_singleplayer": false,
		"is_quick_play_multiplayer":  false,
		"is_quick_play_realms":       false,
	}
	ruleContext := metadata.CurrentRuleContext(features)
	retries := request.DownloadOptions.Retries
	if retries == 0 {
		retries = engine.config.Download.Retries
	}
	installed, err := installFilesWithProgress(operationCtx, engine.downloader, version, gameDirectory,
		versionDirectory, ruleContext, concurrency, retries, request.DownloadOptions.Offline || engine.config.Download.Offline,
		func(progress download.Progress) {
			engine.reportDownload(taskID, progress)
		})
	if err != nil {
		return fail(err)
	}

	engine.updateTask(taskID, models.LaunchPreparing, nil)
	argumentsList, err := arguments.Build(arguments.Options{
		Version: version,
		Install: arguments.InstallPaths{
			GameDirectory: gameDirectory, VersionDirectory: versionDirectory,
			ClientPath: installed.ClientPath, AssetsRoot: installed.AssetsRoot,
			AssetIndex: installed.AssetIndex, NativesPath: installed.NativesPath,
			LogConfig: installed.LogConfig, Classpath: installed.Classpath,
		},
		Request:  request,
		Platform: ruleContext,
	})
	if err != nil {
		var engineError *cleerrors.Error
		if errors.As(err, &engineError) {
			return fail(err)
		}
		return fail(cleerrors.Wrap(cleerrors.CodeLaunchArgumentBuildFailed, "build Minecraft launch arguments", err))
	}
	if request.Account.Type == models.AccountYggdrasil {
		argumentsList, err = addAuthlibInjector(argumentsList, version.MainClass, authlibPath, request.Account.AuthServer)
		if err != nil {
			return fail(err)
		}
	}
	mainClassIndex := len(argumentsList)
	for index, argument := range argumentsList {
		if argument == version.MainClass {
			mainClassIndex = index
			break
		}
	}
	workingDirectory := request.WorkingDirectory
	if workingDirectory == "" {
		workingDirectory = gameDirectory
	}
	engine.mu.Lock()
	if state := engine.tasks[taskID]; state != nil {
		state.debug.MainClass = version.MainClass
		state.debug.JVMArguments = core.RedactArguments(argumentsList[:mainClassIndex])
		if mainClassIndex < len(argumentsList) {
			state.debug.GameArguments = core.RedactArguments(argumentsList[mainClassIndex+1:])
		}
		state.debug.Classpath = append([]string(nil), installed.Classpath...)
		state.debug.WorkingDirectory = workingDirectory
		state.debug.Stage = state.task.Status
	}
	engine.mu.Unlock()
	if operationCtx.Err() != nil {
		return fail(operationCtx.Err())
	}

	engine.updateTask(taskID, models.LaunchLaunching, nil)
	command := exec.Command(java.Path, argumentsList...)
	command.Dir = workingDirectory
	command.Env = environmentWithOverrides(os.Environ(), request.Environment)
	engine.mu.Lock()
	if state := engine.tasks[taskID]; state != nil {
		state.debug.Environment = core.RedactEnvironment(environmentMap(command.Env))
	}
	engine.mu.Unlock()
	stdout, err := command.StdoutPipe()
	if err != nil {
		return fail(cleerrors.Wrap(cleerrors.CodeProcessStartFailed, "create game stdout pipe", err))
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return fail(cleerrors.Wrap(cleerrors.CodeProcessStartFailed, "create game stderr pipe", err))
	}

	engine.mu.Lock()
	state := engine.tasks[taskID]
	if operationCtx.Err() != nil {
		engine.mu.Unlock()
		return fail(operationCtx.Err())
	}
	if err := command.Start(); err != nil {
		engine.mu.Unlock()
		return fail(cleerrors.Wrap(cleerrors.CodeProcessStartFailed, "start Minecraft process", err))
	}
	state.proc = command.Process
	state.done = make(chan struct{})
	state.cancel = nil
	state.task.ProcessID = command.Process.Pid
	state.task.Status = models.LaunchRunning
	engine.publishStatusLocked(state, "")
	engine.emitLocked(core.Event{
		Source: core.EventProcessStarted, TaskID: taskID, ProcessID: command.Process.Pid,
		Level: "INFO", Message: "Minecraft process started",
		Data: map[string]string{"versionId": request.VersionID},
	})
	engine.mu.Unlock()
	engine.logger.Info("Minecraft process started", "taskId", taskID, "processId", command.Process.Pid, "versionId", request.VersionID)
	engine.emit(core.Event{
		Source: core.EventEngineLog, TaskID: taskID, ProcessID: command.Process.Pid,
		Level: "INFO", Message: "Minecraft process started",
	})

	outputDone := make(chan struct{})
	go func() {
		defer close(outputDone)
		var output sync.WaitGroup
		output.Add(2)
		go func() {
			defer output.Done()
			engine.captureOutput(taskID, "stdout", stdout)
		}()
		go func() {
			defer output.Done()
			engine.captureOutput(taskID, "stderr", stderr)
		}()
		output.Wait()
	}()
	go func() {
		<-outputDone
		engine.waitProcess(taskID, command)
	}()
	result, _ := engine.GetLaunchStatus(taskID)
	return result, nil
}

func addAuthlibInjector(argumentsList []string, mainClass, jarPath, apiURL string) ([]string, error) {
	for index, argument := range argumentsList {
		if argument != mainClass {
			continue
		}
		agentArgument := "-javaagent:" + jarPath + "=" + apiURL
		result := make([]string, 0, len(argumentsList)+2)
		result = append(result, argumentsList[:index]...)
		result = append(result, agentArgument, "-Dauthlibinjector.noLogFile")
		result = append(result, argumentsList[index:]...)
		return result, nil
	}
	return nil, cleerrors.New(cleerrors.CodeLaunchArgumentBuildFailed, "Minecraft main class was not found while configuring Authlib Injector")
}

func (engine *Engine) gameDirectory(request models.LaunchRequest) (string, error) {
	directory := request.GameDirectory
	if directory == "" {
		directory = engine.config.Paths.GameDirectory
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return "", cleerrors.Wrap(cleerrors.CodeInvalidGameDirectory, "resolve game directory", err)
	}
	return absolute, nil
}

func (engine *Engine) captureOutput(taskID, stream string, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := core.RedactText(scanner.Text())
		source := core.EventGameStdout
		level := "INFO"
		if stream == "stderr" {
			source = core.EventGameStderr
			level = "ERROR"
		}
		processID := 0
		engine.mu.RLock()
		if state := engine.tasks[taskID]; state != nil {
			processID = state.task.ProcessID
		}
		engine.mu.RUnlock()
		engine.emit(core.Event{
			Source: source, TaskID: taskID, ProcessID: processID, Level: level, Message: line,
			Data: map[string]string{"stream": stream},
		})
		engine.logger.Info("Minecraft output", "taskId", taskID, "stream", stream, "line", line)
	}
	if err := scanner.Err(); err != nil {
		engine.logger.Warn("failed to capture Minecraft output", "taskId", taskID, "stream", stream, "error", err)
		engine.emit(core.Event{
			Source: core.EventEngineLog, TaskID: taskID, Level: "WARN",
			Message: core.RedactText("failed to capture Minecraft " + stream + ": " + err.Error()),
		})
	}
}

func (engine *Engine) waitProcess(taskID string, command *exec.Cmd) {
	err := command.Wait()
	exitCode := 0
	status := models.LaunchExited
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			exitCode = exitError.ExitCode()
		} else {
			status = models.LaunchFailed
		}
	}
	if exitCode != 0 {
		status = models.LaunchFailed
	}
	engine.mu.Lock()
	state, ok := engine.tasks[taskID]
	if ok {
		if state.stopped {
			status = models.LaunchStopped
		}
		previous := state.task.Status
		if status == models.LaunchFailed {
			state.task.FailedAtStage = previous
		}
		state.task.Status = status
		state.task.FinishedAt = time.Now().UTC()
		state.task.ExitCode = &exitCode
		state.proc = nil
		state.downloads = nil
		if state.done != nil {
			close(state.done)
			state.done = nil
		}
		if err != nil {
			state.task.Error = core.RedactText(err.Error())
			if exitCode != 0 && !state.stopped {
				state.task.ErrorCode = string(cleerrors.CodeProcessStartFailed)
			}
		}
		if state.stopped {
			state.task.ErrorCode = ""
		}
		if previous != status {
			engine.publishStatusLocked(state, state.task.Error)
		}
		engine.emitLocked(core.Event{
			Source: core.EventProcessExited, TaskID: taskID, ProcessID: state.task.ProcessID,
			Level: exitLevelForCode(exitCode), Message: fmt.Sprintf("Minecraft process exited with code %d", exitCode),
			Data: map[string]string{"exitCode": fmt.Sprint(exitCode), "status": string(status)},
		})
	}
	engine.mu.Unlock()
	exitLevel := "INFO"
	if err != nil {
		exitLevel = "ERROR"
		engine.logger.Warn("Minecraft process exited", "taskId", taskID, "exitCode", exitCode, "error", err)
	} else {
		engine.logger.Info("Minecraft process exited", "taskId", taskID, "exitCode", exitCode)
	}
	engine.emit(core.Event{
		Source: core.EventEngineLog, TaskID: taskID, Level: exitLevel,
		Message: fmt.Sprintf("Minecraft process exited with code %d", exitCode),
	})
}

func (engine *Engine) updateTask(taskID string, status models.LaunchStatus, err error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	state, ok := engine.tasks[taskID]
	if !ok {
		return
	}
	previous := state.task.Status
	if status == models.LaunchFailed {
		state.task.FailedAtStage = previous
	}
	state.task.Status = status
	if status == models.LaunchFailed || status == models.LaunchCancelled {
		state.task.FinishedAt = time.Now().UTC()
		state.downloads = nil
	}
	if err != nil {
		state.task.Error = core.RedactText(err.Error())
		state.task.ErrorCode = ""
		var engineError *cleerrors.Error
		if errors.As(err, &engineError) {
			state.task.ErrorCode = string(engineError.Code)
		}
	}
	if previous != status {
		message := ""
		if err != nil {
			message = core.RedactText(err.Error())
		}
		engine.publishStatusLocked(state, message)
	}
	state.debug.Task = state.task
	state.debug.Stage = status
}

func (engine *Engine) GetLaunchStatus(taskID string) (models.LaunchTask, error) {
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	state, ok := engine.tasks[taskID]
	if !ok {
		return models.LaunchTask{}, cleerrors.New(cleerrors.CodeTaskNotFound, "launch task was not found")
	}
	task := state.task
	if state.task.ExitCode != nil {
		exitCode := *state.task.ExitCode
		task.ExitCode = &exitCode
	}
	return task, nil
}

func (engine *Engine) SubscribeTask(taskID string) (<-chan models.LaunchStatusEvent, func(), error) {
	engine.mu.Lock()
	if engine.closed {
		engine.mu.Unlock()
		return nil, nil, errors.New("Minecraft engine is closed")
	}
	state, ok := engine.tasks[taskID]
	if !ok {
		engine.mu.Unlock()
		return nil, nil, cleerrors.New(cleerrors.CodeTaskNotFound, "launch task was not found")
	}
	subscriptionID := engine.nextSubID.Add(1)
	events := make(chan models.LaunchStatusEvent, 32)
	if engine.subscribers[taskID] == nil {
		engine.subscribers[taskID] = make(map[uint64]chan models.LaunchStatusEvent)
	}
	engine.subscribers[taskID][subscriptionID] = events
	events <- models.LaunchStatusEvent{
		TaskID: taskID, Status: state.task.Status, Timestamp: time.Now().UTC(),
	}
	engine.mu.Unlock()

	unsubscribe := func() {
		engine.mu.Lock()
		if subscribers := engine.subscribers[taskID]; subscribers != nil {
			if current, exists := subscribers[subscriptionID]; exists {
				delete(subscribers, subscriptionID)
				close(current)
			}
			if len(subscribers) == 0 {
				delete(engine.subscribers, taskID)
			}
		}
		engine.mu.Unlock()
	}
	return events, unsubscribe, nil
}

func (engine *Engine) publishStatusLocked(state *taskState, message string) {
	event := models.LaunchStatusEvent{
		TaskID: state.task.ID, Status: state.task.Status,
		Timestamp: time.Now().UTC(), Message: message,
	}
	for _, subscriber := range engine.subscribers[state.task.ID] {
		select {
		case subscriber <- event:
		default:
			select {
			case <-subscriber:
			default:
			}
			select {
			case subscriber <- event:
			default:
			}
		}
	}
	eventMessage := "Launch task status changed to " + string(state.task.Status)
	data := map[string]string{"status": string(state.task.Status)}
	if message != "" {
		eventMessage += ": " + core.RedactText(message)
	}
	if state.task.ErrorCode != "" {
		data["errorCode"] = state.task.ErrorCode
	}
	if state.task.FailedAtStage != "" {
		data["failedAtStage"] = string(state.task.FailedAtStage)
	}
	level := "INFO"
	if state.task.Status == models.LaunchFailed {
		level = "ERROR"
	} else if state.task.Status == models.LaunchCancelled || state.task.Status == models.LaunchStopped {
		level = "WARN"
	}
	engine.emitLocked(core.Event{
		Source: core.EventTaskStatus, TaskID: state.task.ID, ProcessID: state.task.ProcessID,
		Level: level, Message: eventMessage, Data: data,
	})
}

func exitLevelForCode(code int) string {
	if code == 0 {
		return "INFO"
	}
	return "ERROR"
}

func (engine *Engine) CancelTask(_ context.Context, taskID string) error {
	engine.mu.Lock()
	state, ok := engine.tasks[taskID]
	if !ok {
		engine.mu.Unlock()
		return cleerrors.New(cleerrors.CodeTaskNotFound, "launch task was not found")
	}
	cancel := state.cancel
	process := state.proc
	status := state.task.Status
	engine.mu.Unlock()

	if process != nil {
		return cleerrors.New(cleerrors.CodeProcessStartFailed, "Minecraft is already running; stop the process instead of cancelling its launch task")
	}
	if status == models.LaunchExited || status == models.LaunchFailed || status == models.LaunchCancelled || status == models.LaunchStopped {
		return cleerrors.New(cleerrors.CodeTaskNotFound, "launch task is already complete")
	}
	if cancel != nil {
		cancel()
	}
	return nil
}

func (engine *Engine) Close() error {
	engine.mu.Lock()
	if engine.closed {
		engine.mu.Unlock()
		return nil
	}
	engine.closed = true
	type closingProcess struct {
		process    *os.Process
		done       <-chan struct{}
		shouldStop bool
	}
	processes := make([]closingProcess, 0, len(engine.tasks))
	for _, state := range engine.tasks {
		if state.task.Status == models.LaunchRunning {
			shouldStop := !state.stopped
			state.stopped = true
			if state.proc != nil {
				processes = append(processes, closingProcess{
					process: state.proc, done: state.done, shouldStop: shouldStop,
				})
			}
		} else if state.cancel != nil {
			state.cancel()
		}
	}
	for taskID, subscribers := range engine.subscribers {
		for _, subscriber := range subscribers {
			close(subscriber)
		}
		delete(engine.subscribers, taskID)
	}
	for subscriptionID, subscriber := range engine.eventSubs {
		close(subscriber)
		delete(engine.eventSubs, subscriptionID)
	}
	engine.mu.Unlock()

	var firstErr error
	for _, current := range processes {
		if current.process != nil && current.shouldStop {
			if err := stopProcess(current.process); err != nil && !errors.Is(err, os.ErrProcessDone) && firstErr == nil {
				firstErr = err
			}
		}
		if current.done != nil {
			timer := time.NewTimer(5 * time.Second)
			select {
			case <-current.done:
			case <-timer.C:
				if firstErr == nil {
					firstErr = fmt.Errorf("Minecraft process %d did not exit after the graceful stop request", current.process.Pid)
				}
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		}
	}
	return firstErr
}

func environmentWithOverrides(environment []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return environment
	}
	result := make([]string, 0, len(environment)+len(overrides))
	for _, entry := range environment {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, overridden := overrides[key]; !overridden {
			result = append(result, entry)
		}
	}
	for key, value := range overrides {
		result = append(result, key+"="+value)
	}
	return result
}

func environmentMap(environment []string) map[string]string {
	values := make(map[string]string, len(environment))
	for _, entry := range environment {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	return values
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
