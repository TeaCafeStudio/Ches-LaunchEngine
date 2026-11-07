package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/config"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/accounts"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/core"
	cleerrors "github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/errors"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

func Parse(line string) ([]string, error) {
	var words []string
	var current strings.Builder
	var quote rune
	escaped := false
	active := false
	characters := []rune(line)
	for index := 0; index < len(characters); index++ {
		character := characters[index]
		if escaped {
			current.WriteRune(character)
			escaped = false
			active = true
			continue
		}
		if character == '\\' && quote != '\'' {
			if index+1 == len(characters) {
				return nil, errors.New("command line ends with an incomplete escape")
			}
			next := characters[index+1]
			if next == '\\' || next == '"' || next == '\'' || next == ' ' || next == '\t' {
				escaped = true
			} else {
				current.WriteRune(character)
			}
			active = true
			continue
		}
		if quote != 0 {
			if character == quote {
				quote = 0
			} else {
				current.WriteRune(character)
			}
			active = true
			continue
		}
		switch character {
		case '\'', '"':
			quote = character
			active = true
		case ' ', '\t', '\r', '\n':
			if active {
				words = append(words, current.String())
				current.Reset()
				active = false
			}
		default:
			current.WriteRune(character)
			active = true
		}
	}
	if escaped {
		return nil, errors.New("command line ends with an incomplete escape")
	}
	if quote != 0 {
		return nil, errors.New("command line has an unterminated quote")
	}
	if active {
		words = append(words, current.String())
	}
	return words, nil
}

func (terminal *Terminal) registerCommands() error {
	commands := []struct {
		name    string
		handler Handler
	}{
		{"help", commandHelp},
		{"clear", commandClear},
		{"version", commandVersion},
		{"exit", commandExit},
		{"quit", commandExit},
		{"launch", commandLaunch},
		{"status", commandStatus},
		{"tasks", commandTasks},
		{"cancel", commandCancel},
		{"process", commandProcess},
		{"stop", commandStop},
		{"java", commandJava},
		{"download", commandDownload},
		{"logs", commandLogs},
		{"watch", commandWatch},
		{"unwatch", commandUnwatch},
		{"debug", commandDebug},
		{"account", commandAccount},
	}
	for _, command := range commands {
		if err := terminal.registry.Register(command.name, command.handler); err != nil {
			return err
		}
	}
	return nil
}

func commandHelp(terminal *Terminal, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: help")
	}
	terminal.printLine("Commands:")
	terminal.printLine("  help | clear | version | exit | quit")
	terminal.printLine("  launch --config <request.json> | status [task-id] | tasks | cancel <task-id>")
	terminal.printLine("  process [list] | process inspect <pid> | stop <pid> [--force]")
	terminal.printLine("  version list | version inspect <id> | version resolve <id>")
	terminal.printLine("  java list | java detect | java inspect <java-path>")
	terminal.printLine("  download status | download list | download cancel <task-id>")
	terminal.printLine("  logs [--task <id>|--process <pid>] [--limit <n>] [--follow]")
	terminal.printLine("  watch [<task-id>|<pid>] | unwatch")
	terminal.printLine("  debug info <task-id> | debug config | debug arguments|classpath|environment <task-id>")
	terminal.printLine("  account list | add offline <name> [uuid] | add yggdrasil <auth-server-url> <username> | add microsoft")
	terminal.printLine("  account inspect|profile|validate|refresh|logout|remove|switch <id>")
	terminal.printLine("Ctrl+C cancels the active launch, gracefully stops its running process, or exits.")
	return nil
}

func commandClear(terminal *Terminal, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: clear")
	}
	terminal.mu.Lock()
	if terminal.interactive {
		_, _ = fmt.Fprint(terminal.output, "\x1b[2J\x1b[H")
	} else {
		_, _ = fmt.Fprintln(terminal.output)
	}
	terminal.mu.Unlock()
	return nil
}

func commandVersion(terminal *Terminal, args []string) error {
	if len(args) == 0 {
		terminal.printLine("ChesLaunchEngine " + terminal.version)
		return nil
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return errors.New("usage: version list")
		}
		versions, err := terminal.engine.ListVersions(terminal.context())
		if err != nil {
			return err
		}
		if len(versions) == 0 {
			terminal.printLine("No version manifest is available in the configured cache.")
			return nil
		}
		limit := len(versions)
		if limit > 25 {
			limit = 25
		}
		for _, version := range versions[:limit] {
			terminal.printLine(fmt.Sprintf("%-16s %-10s %s", version.ID, version.Type, version.ReleaseTime.Format(time.RFC3339)))
		}
		if limit < len(versions) {
			terminal.printLine(fmt.Sprintf("Showing %d of %d versions.", limit, len(versions)))
		}
		return nil
	case "inspect", "resolve":
		if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: version %s <version-id>", args[0])
		}
		version, err := terminal.engine.InspectVersion(terminal.context(), args[1])
		if err != nil {
			return err
		}
		terminal.printLine(fmt.Sprintf("ID: %s\nType: %s\nMain class: %s\nJava major: %d\nAssets: %s\nLibraries: %d",
			version.ID, version.Type, version.MainClass, version.JavaMajor, version.AssetIndex, version.LibraryCount))
		if args[0] == "resolve" {
			terminal.printLine("Version metadata resolved successfully.")
		}
		return nil
	default:
		return errors.New("usage: version [list|inspect <id>|resolve <id>]")
	}
}

func commandExit(_ *Terminal, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: exit")
	}
	return ErrExit
}

func commandLaunch(terminal *Terminal, args []string) error {
	path := terminal.defaultLaunchConfig
	if len(args) == 2 && args[0] == "--config" {
		path = args[1]
	} else if len(args) != 0 {
		return errors.New("usage: launch [--config <request.json>]")
	}
	if strings.TrimSpace(path) == "" {
		return errors.New("launch request is required; use launch --config <request.json>")
	}
	request, err := config.LoadLaunchRequest(path)
	if err != nil {
		return err
	}
	task, err := terminal.engine.StartLaunch(terminal.context(), request)
	if err != nil {
		return err
	}

	terminal.mu.Lock()
	terminal.currentTaskID = task.ID
	terminal.mu.Unlock()
	terminal.printLine(fmt.Sprintf(
		"Launch Task ID: %s\nMinecraft version: %s\nJava path: %s\nAccount type: %s\nGame directory: %s\nLoader: %s\nStage: %s\nDownloads: 0/0\nProcess PID: pending",
		task.ID, firstValue(task.VersionID, request.VersionID),
		firstValue(request.JavaPath, terminal.cfg.Java.Path, "detecting"),
		firstValue(string(task.AccountType), string(request.Account.Type), "offline"),
		firstValue(task.GameDirectory, request.GameDirectory, terminal.cfg.Paths.GameDirectory, "(default)"),
		firstValue(string(task.LoaderType), string(request.Loader.Type), "vanilla"), task.Status,
	))
	terminal.mu.Lock()
	nonInteractive := terminal.nonInteractive
	terminal.mu.Unlock()
	if nonInteractive {
		return terminal.waitForLaunchTask(task.ID)
	}
	return nil
}

func (terminal *Terminal) waitForLaunchTask(taskID string) error {
	events, unsubscribe, err := terminal.engine.SubscribeTask(taskID)
	if err != nil {
		return fmt.Errorf("subscribe to launch task: %w", err)
	}
	defer unsubscribe()

	ctx := terminal.context()
	for {
		select {
		case <-ctx.Done():
			return nil
		case event, open := <-events:
			if !open {
				return errors.New("launch task subscription closed before completion")
			}
			switch event.Status {
			case models.LaunchExited, models.LaunchStopped, models.LaunchFailed, models.LaunchCancelled:
				return commandStatus(terminal, []string{taskID})
			}
		}
	}
}

func commandAccount(terminal *Terminal, args []string) error {
	if terminal.accounts == nil {
		return errors.New("account management is unavailable")
	}
	ctx := terminal.context()
	if len(args) == 1 && args[0] == "list" {
		accounts, err := terminal.accounts.ListAccounts(ctx)
		if err != nil {
			return err
		}
		if len(accounts) == 0 {
			terminal.printLine("No accounts are stored.")
			return nil
		}
		active, activeErr := terminal.accounts.ActiveAccount(ctx)
		for _, account := range accounts {
			marker := ""
			if activeErr == nil && account.ID == active.ID {
				marker = " (active)"
			}
			terminal.printLine(fmt.Sprintf("%s %s %s%s", account.ID, account.Type, account.Name, marker))
		}
		return nil
	}
	if len(args) >= 2 && args[0] == "add" {
		if args[1] == "yggdrasil" {
			if len(args) != 4 {
				return errors.New("usage: account add yggdrasil <auth-server-url> <username>")
			}
			password, err := terminal.readSecret("Yggdrasil password (hidden): ")
			if err != nil {
				return err
			}
			request := models.AccountAuthRequest{
				Type: models.AccountYggdrasil, AuthServer: args[2], Username: args[3],
				Password: string(password),
			}
			zeroBytes(password)
			account, authErr := terminal.accounts.AuthenticateAccount(ctx, request)
			request.Password = ""
			if authErr != nil {
				return authErr
			}
			terminal.printLine(fmt.Sprintf("Created Yggdrasil account %s (%s), UUID %s.", account.Name, account.ID, account.UUID))
			return nil
		}
		if args[1] == "microsoft" {
			if len(args) != 2 {
				return errors.New("usage: account add microsoft")
			}
			provider, err := terminal.accounts.Provider(models.AccountMicrosoft)
			if err != nil {
				return err
			}
			auth, ok := provider.(accounts.DeviceCodeAuthenticator)
			if !ok {
				return errors.New("Microsoft account provider is unavailable")
			}
			deviceCode, err := auth.StartDeviceCode(ctx)
			if err != nil {
				return err
			}
			terminal.printLine(fmt.Sprintf("Open %s and enter the code %s", deviceCode.VerificationURL, deviceCode.UserCode))
			terminal.printLine("Waiting for Microsoft authorization (cancel with Ctrl+C)...")
			account, err := auth.AuthenticateDeviceCode(ctx, deviceCode)
			if err != nil {
				return err
			}
			stored, err := terminal.accounts.AddAccount(ctx, account)
			if err != nil {
				return err
			}
			terminal.printLine(fmt.Sprintf("Created Microsoft account %s (%s), UUID %s.", stored.Name, stored.ID, stored.UUID))
			return nil
		}
		if args[1] != "offline" || (len(args) != 3 && len(args) != 4) {
			return errors.New("usage: account add offline <name> [uuid] | account add yggdrasil <auth-server-url> <username> | account add microsoft")
		}
		request := models.AccountAuthRequest{Type: models.AccountOffline, Username: args[2]}
		if len(args) == 4 {
			request.UUID = args[3]
		}
		account, err := terminal.accounts.AuthenticateAccount(ctx, request)
		if err != nil {
			return err
		}
		terminal.printLine(fmt.Sprintf("Created offline account %s (%s), UUID %s.", account.Name, account.ID, account.UUID))
		return nil
	}
	if len(args) != 2 {
		return errors.New("usage: account list | add offline <name> [uuid] | add yggdrasil <auth-server-url> <username> | add microsoft | inspect|profile|validate|refresh|logout|remove|switch <id>")
	}
	id := args[1]
	switch args[0] {
	case "inspect":
		account, err := terminal.accounts.GetAccount(ctx, id)
		if err != nil {
			return err
		}
		terminal.printLine(fmt.Sprintf("ID: %s\nType: %s\nUsername: %s\nDisplay name: %s\nUUID: %s\nCreated: %s\nUpdated: %s",
			account.ID, account.Type, account.Name, account.DisplayName, account.UUID,
			account.CreatedAt.Format(time.RFC3339), account.UpdatedAt.Format(time.RFC3339)))
	case "profile":
		profile, err := terminal.accounts.GetAccountProfile(ctx, id)
		if err != nil {
			return err
		}
		terminal.printLine(fmt.Sprintf("Profile: %s (%s), properties: %d", profile.Name, profile.ID, len(profile.Properties)))
	case "validate":
		if err := terminal.accounts.ValidateAccount(ctx, id); err != nil {
			return err
		}
		terminal.printLine("Account profile is valid.")
	case "refresh":
		account, err := terminal.accounts.RefreshAccount(ctx, id)
		if err != nil {
			return err
		}
		terminal.printLine("Account refreshed: " + account.ID)
	case "logout":
		if err := terminal.accounts.LogoutAccount(ctx, id); err != nil {
			return err
		}
		terminal.printLine("Account session logged out.")
	case "remove":
		if err := terminal.accounts.RemoveAccount(ctx, id); err != nil {
			return err
		}
		terminal.printLine("Account removed.")
	case "switch":
		if err := terminal.accounts.SetActiveAccount(ctx, id); err != nil {
			return err
		}
		terminal.printLine("Active account set to " + id)
	default:
		return errors.New("usage: account list | add offline <name> [uuid] | add yggdrasil <auth-server-url> <username> | add microsoft | inspect|profile|validate|refresh|logout|remove|switch <id>")
	}
	return nil
}

func LoadLaunchRequest(path string) (models.LaunchRequest, error) {
	return config.LoadLaunchRequest(path)
}

func commandStatus(terminal *Terminal, args []string) error {
	if len(args) > 1 {
		return errors.New("usage: status [task-id]")
	}
	if len(args) == 0 {
		return commandTasks(terminal, nil)
	}
	task, err := terminal.engine.GetLaunchStatus(args[0])
	if err != nil {
		return err
	}
	terminal.printLine(formatTask(task))
	if task.Status == models.LaunchFailed {
		terminal.printLine("Error code: " + firstValue(task.ErrorCode, "UNKNOWN"))
		terminal.printLine("Error: " + task.Error)
		terminal.printLine("Stage: " + firstValue(string(task.FailedAtStage), string(task.Status)))
		terminal.printLine("Likely cause: " + diagnosticHint(task.ErrorCode))
		for _, event := range terminal.engine.RecentEvents(core.EventFilter{TaskID: task.ID}, 12) {
			terminal.printEvent(event)
		}
	}
	return nil
}

func commandTasks(terminal *Terminal, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: tasks")
	}
	tasks := terminal.engine.ListLaunchTasks()
	if len(tasks) == 0 {
		terminal.printLine("No launch tasks.")
		return nil
	}
	for _, task := range tasks {
		terminal.printLine(formatTask(task))
	}
	return nil
}

func formatTask(task models.LaunchTask) string {
	pid := "-"
	if task.ProcessID != 0 {
		pid = strconv.Itoa(task.ProcessID)
	}
	return fmt.Sprintf("%s version=%s stage=%s pid=%s exit=%s", task.ID, task.VersionID, task.Status, pid, exitValue(task.ExitCode))
}

func exitValue(code *int) string {
	if code == nil {
		return "pending"
	}
	return strconv.Itoa(*code)
}

func diagnosticHint(code string) string {
	switch cleerrors.Code(code) {
	case cleerrors.CodeJavaNotFound:
		return "configure Java or ensure its executable is on PATH"
	case cleerrors.CodeJavaVersionIncompatible:
		return "install the Java major version required by this Minecraft release"
	case cleerrors.CodeVersionJSONInvalid:
		return "check the version ID, network access, and cached metadata"
	case cleerrors.CodeLibraryDownloadFailed, cleerrors.CodeAssetDownloadFailed, cleerrors.CodeHashMismatch:
		return "check network access, disk space, and the download cache"
	case cleerrors.CodeProcessStartFailed:
		return "check the Java executable, launch arguments, and game logs"
	default:
		return "inspect recent engine and game log events for this task"
	}
}

func commandCancel(terminal *Terminal, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: cancel <task-id>")
	}
	return terminal.engine.CancelTask(terminal.context(), args[0])
}

func commandProcess(terminal *Terminal, args []string) error {
	if len(args) == 0 || (len(args) == 1 && args[0] == "list") {
		processes := terminal.engine.ListProcesses()
		if len(processes) == 0 {
			terminal.printLine("No managed Minecraft processes.")
			return nil
		}
		for _, process := range processes {
			terminal.printLine(formatProcess(process))
		}
		return nil
	}
	if len(args) == 2 && args[0] == "inspect" {
		processID, err := strconv.Atoi(args[1])
		if err != nil || processID < 1 {
			return errors.New("process ID must be a positive integer")
		}
		process, err := terminal.engine.InspectProcess(processID)
		if err != nil {
			return err
		}
		terminal.printLine(formatProcess(process))
		return nil
	}
	return errors.New("usage: process [list|inspect <process-id>]")
}

func formatProcess(process core.ProcessInfo) string {
	return fmt.Sprintf("pid=%d task=%s version=%s status=%s exit=%s",
		process.ProcessID, process.TaskID, process.VersionID, process.Status, exitValue(process.ExitCode))
}

func commandStop(terminal *Terminal, args []string) error {
	force := false
	var id string
	switch len(args) {
	case 1:
		id = args[0]
	case 2:
		if args[0] == "--force" {
			force, id = true, args[1]
		} else if args[1] == "--force" {
			id, force = args[0], true
		} else {
			return errors.New("usage: stop <process-id> [--force]")
		}
	default:
		return errors.New("usage: stop <process-id> [--force]")
	}
	processID, err := strconv.Atoi(id)
	if err != nil || processID < 1 {
		return errors.New("process ID must be a positive integer")
	}
	if err := terminal.engine.StopProcess(terminal.context(), processID, force); err != nil {
		return err
	}
	if force {
		terminal.printLine(fmt.Sprintf("Force stop requested for process %d.", processID))
	} else {
		terminal.printLine(fmt.Sprintf("Graceful stop requested for process %d.", processID))
	}
	return nil
}

func commandJava(terminal *Terminal, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: java list|detect|inspect <java-path>")
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return errors.New("usage: java list")
		}
		runtimes, err := terminal.engine.ListJava(terminal.context())
		if err != nil {
			return err
		}
		if len(runtimes) == 0 {
			terminal.printLine("No usable Java runtime was found.")
			return nil
		}
		for _, java := range runtimes {
			terminal.printLine(fmt.Sprintf("Java %s (major %d, %s): %s", java.Version, java.Major, java.Architecture, java.Path))
		}
	case "detect", "inspect":
		if (args[0] == "detect" && len(args) != 1) || (args[0] == "inspect" && len(args) != 2) {
			return errors.New("usage: java detect | java inspect <java-path>")
		}
		path := ""
		if len(args) == 2 {
			path = args[1]
		}
		java, err := terminal.engine.DetectJava(terminal.context(), path)
		if err != nil {
			return err
		}
		terminal.printLine(fmt.Sprintf("Java %s (major %d, %s): %s", java.Version, java.Major, java.Architecture, java.Path))
	default:
		return errors.New("usage: java list|detect|inspect <java-path>")
	}
	return nil
}

func commandDownload(terminal *Terminal, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: download status|list|cancel <task-id>")
	}
	switch args[0] {
	case "status":
		if len(args) != 1 {
			return errors.New("usage: download status")
		}
		found := false
		for _, task := range terminal.engine.ListLaunchTasks() {
			if task.Status == models.LaunchDownloading {
				found = true
				info, err := terminal.engine.GetLaunchDebugInfo(task.ID)
				if err != nil {
					return err
				}
				terminal.printLine(fmt.Sprintf("task=%s downloads=%d/%d stage=%s", task.ID, info.DownloadsDone, info.DownloadsTotal, task.Status))
			}
		}
		if !found {
			terminal.printLine("No active downloads.")
		}
	case "list":
		if len(args) != 1 {
			return errors.New("usage: download list")
		}
		events := terminal.engine.RecentEvents(core.EventFilter{}, 100)
		count := 0
		for _, event := range events {
			if event.Source != core.EventDownload {
				continue
			}
			terminal.printEvent(event)
			count++
		}
		if count == 0 {
			terminal.printLine("No download progress has been recorded.")
		}
	case "cancel":
		if len(args) != 2 {
			return errors.New("usage: download cancel <task-id>")
		}
		return terminal.engine.CancelDownloads(terminal.context(), args[1])
	default:
		return errors.New("usage: download status|list|cancel <task-id>")
	}
	return nil
}

func commandLogs(terminal *Terminal, args []string) error {
	filter := core.EventFilter{}
	limit := 100
	follow := false
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--task":
			index++
			if index >= len(args) {
				return errors.New("--task requires a task ID")
			}
			filter.TaskID = args[index]
		case "--process":
			index++
			if index >= len(args) {
				return errors.New("--process requires a process ID")
			}
			processID, err := strconv.Atoi(args[index])
			if err != nil || processID < 1 {
				return errors.New("process ID must be a positive integer")
			}
			filter.ProcessID = processID
		case "--limit":
			index++
			if index >= len(args) {
				return errors.New("--limit requires a number")
			}
			var err error
			limit, err = strconv.Atoi(args[index])
			if err != nil || limit < 1 || limit > 2000 {
				return errors.New("log limit must be between 1 and 2000")
			}
		case "--follow":
			follow = true
		default:
			return errors.New("usage: logs [--task <id>|--process <pid>] [--limit <n>] [--follow]")
		}
	}
	if follow {
		terminal.setWatch(filter, true)
		terminal.printLine("Following matching events; use unwatch to stop.")
	}
	events := terminal.engine.RecentEvents(filter, limit)
	if len(events) == 0 {
		terminal.printLine("No recent events match that filter.")
		return nil
	}
	for _, event := range events {
		terminal.printEvent(event)
	}
	return nil
}

func commandWatch(terminal *Terminal, args []string) error {
	filter := core.EventFilter{}
	if len(args) > 1 {
		return errors.New("usage: watch [<task-id>|<process-id>]")
	}
	if len(args) == 1 {
		if processID, err := strconv.Atoi(args[0]); err == nil {
			if processID < 1 {
				return errors.New("process ID must be a positive integer")
			}
			if _, err := terminal.engine.InspectProcess(processID); err != nil {
				return err
			}
			filter.ProcessID = processID
		} else {
			if _, err := terminal.engine.GetLaunchStatus(args[0]); err != nil {
				return err
			}
			filter.TaskID = args[0]
		}
	}
	terminal.setWatch(filter, true)
	if len(args) == 0 {
		terminal.printLine("Watching events from all tasks; use unwatch to stop.")
	} else {
		terminal.printLine("Watching events for " + args[0] + "; use unwatch to stop.")
	}
	return nil
}

func commandUnwatch(terminal *Terminal, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: unwatch")
	}
	terminal.setWatch(core.EventFilter{}, false)
	terminal.printLine("Stopped displaying watched events.")
	return nil
}

func (terminal *Terminal) setWatch(filter core.EventFilter, enabled bool) {
	terminal.mu.Lock()
	terminal.watchFilter = filter
	terminal.watchEnabled = enabled
	terminal.displayEvents = enabled
	terminal.mu.Unlock()
}

func commandDebug(terminal *Terminal, args []string) error {
	if len(args) == 1 && args[0] == "config" {
		data, err := json.MarshalIndent(terminal.cfg, "", "  ")
		if err != nil {
			return fmt.Errorf("format CLE configuration: %w", err)
		}
		terminal.printLine(string(data))
		return nil
	}
	if len(args) == 2 && args[0] == "info" {
		info, err := terminal.engine.GetLaunchDebugInfo(args[1])
		if err != nil {
			return err
		}
		terminal.printLine(fmt.Sprintf("task=%s version=%s java=%s account=%s gameDirectory=%s loader=%s stage=%s pid=%d downloads=%d/%d",
			info.Task.ID, info.VersionID, info.JavaPath, info.AccountType,
			info.GameDirectory, info.LoaderType, info.Stage, info.Task.ProcessID,
			info.DownloadsDone, info.DownloadsTotal))
		return nil
	}
	if len(args) != 2 {
		return errors.New("usage: debug info <task-id> | debug config | debug arguments|classpath|environment <task-id>")
	}
	info, err := terminal.engine.GetLaunchDebugInfo(args[1])
	if err != nil {
		return err
	}
	switch args[0] {
	case "arguments":
		terminal.printLine("Java executable: " + info.JavaPath)
		terminal.printLine("JVM arguments:")
		for _, argument := range info.JVMArguments {
			terminal.printLine("  " + argument)
		}
		terminal.printLine("Game arguments:")
		terminal.printLine("  Main class: " + info.MainClass)
		for _, argument := range info.GameArguments {
			terminal.printLine("  " + argument)
		}
		terminal.printLine("Working directory: " + info.WorkingDirectory)
	case "classpath":
		terminal.printLine(fmt.Sprintf("Classpath entries: %d", len(info.Classpath)))
		for _, entry := range info.Classpath {
			terminal.printLine("  " + entry)
		}
	case "environment":
		keys := make([]string, 0, len(info.Environment))
		for key := range info.Environment {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		terminal.printLine("Environment overrides:")
		for _, key := range keys {
			terminal.printLine("  " + key + "=" + info.Environment[key])
		}
	default:
		return errors.New("usage: debug info <task-id> | debug config | debug arguments|classpath|environment <task-id>")
	}
	return nil
}

func (terminal *Terminal) context() context.Context {
	terminal.mu.Lock()
	defer terminal.mu.Unlock()
	if terminal.ctx != nil {
		return terminal.ctx
	}
	return context.Background()
}

func (terminal *Terminal) printErrorDetails(err error) string {
	var engineError *cleerrors.Error
	if errors.As(err, &engineError) {
		return fmt.Sprintf("%s: %s", engineError.Code, core.RedactText(engineError.Message))
	}
	return core.RedactText(err.Error())
}

func firstValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
