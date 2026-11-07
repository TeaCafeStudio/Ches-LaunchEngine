package terminal

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/config"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/accounts"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/core"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

type testEngine struct {
	mu             sync.Mutex
	tasks          map[string]models.LaunchTask
	processes      []core.ProcessInfo
	events         []core.Event
	startedRequest models.LaunchRequest
	finishStatus   models.LaunchStatus
	cancelled      []string
	stopped        []int
	unsubscribed   int
}

func newTestEngine() *testEngine {
	return &testEngine{tasks: make(map[string]models.LaunchTask)}
}

func (engine *testEngine) Validate(context.Context, models.LaunchRequest) error { return nil }
func (engine *testEngine) Launch(_ context.Context, request models.LaunchRequest) (models.LaunchTask, error) {
	engine.startedRequest = request
	task := models.LaunchTask{ID: "task-1", Status: models.LaunchRunning, VersionID: request.VersionID}
	engine.tasks[task.ID] = task
	return task, nil
}
func (engine *testEngine) StartLaunch(_ context.Context, request models.LaunchRequest) (models.LaunchTask, error) {
	engine.startedRequest = request
	task := models.LaunchTask{ID: "task-1", Status: models.LaunchResolving, VersionID: request.VersionID}
	engine.tasks[task.ID] = task
	return task, nil
}
func (engine *testEngine) GetLaunchStatus(id string) (models.LaunchTask, error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	task, ok := engine.tasks[id]
	if !ok {
		return models.LaunchTask{}, errors.New("task not found")
	}
	return task, nil
}
func (engine *testEngine) SubscribeTask(id string) (<-chan models.LaunchStatusEvent, func(), error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	events := make(chan models.LaunchStatusEvent, 1)
	status := engine.tasks[id].Status
	if engine.finishStatus != "" {
		status = engine.finishStatus
		task := engine.tasks[id]
		task.Status = status
		engine.tasks[id] = task
	}
	events <- models.LaunchStatusEvent{TaskID: id, Status: status}
	close(events)
	return events, func() {}, nil
}
func (engine *testEngine) CancelTask(_ context.Context, id string) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.cancelled = append(engine.cancelled, id)
	task := engine.tasks[id]
	task.Status = models.LaunchCancelled
	engine.tasks[id] = task
	return nil
}
func (engine *testEngine) Close() error { return nil }
func (engine *testEngine) SubscribeEvents(buffer int) (<-chan core.Event, func(), error) {
	channel := make(chan core.Event, buffer)
	return channel, func() {
		engine.mu.Lock()
		engine.unsubscribed++
		engine.mu.Unlock()
	}, nil
}
func (engine *testEngine) RecentEvents(filter core.EventFilter, limit int) []core.Event {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	var result []core.Event
	for _, event := range engine.events {
		if filter.TaskID != "" && filter.TaskID != event.TaskID {
			continue
		}
		if filter.ProcessID != 0 && filter.ProcessID != event.ProcessID {
			continue
		}
		result = append(result, event)
	}
	if len(result) > limit {
		result = result[len(result)-limit:]
	}
	return result
}
func (engine *testEngine) ListLaunchTasks() []models.LaunchTask {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	result := make([]models.LaunchTask, 0, len(engine.tasks))
	for _, task := range engine.tasks {
		result = append(result, task)
	}
	return result
}
func (engine *testEngine) ListProcesses() []core.ProcessInfo { return engine.processes }
func (engine *testEngine) InspectProcess(id int) (core.ProcessInfo, error) {
	for _, process := range engine.processes {
		if process.ProcessID == id {
			return process, nil
		}
	}
	return core.ProcessInfo{}, errors.New("process not found")
}
func (engine *testEngine) StopProcess(_ context.Context, id int, _ bool) error {
	engine.stopped = append(engine.stopped, id)
	return nil
}
func (engine *testEngine) CancelDownloads(context.Context, string) error { return nil }
func (engine *testEngine) GetLaunchDebugInfo(id string) (core.LaunchDebugInfo, error) {
	task, err := engine.GetLaunchStatus(id)
	if err != nil {
		return core.LaunchDebugInfo{}, err
	}
	return core.LaunchDebugInfo{Task: task, VersionID: task.VersionID}, nil
}
func (engine *testEngine) ListVersions(context.Context) ([]core.VersionInfo, error) {
	return []core.VersionInfo{{ID: "1.21.1", Type: "release"}}, nil
}
func (engine *testEngine) InspectVersion(_ context.Context, id string) (core.VersionInfo, error) {
	return core.VersionInfo{ID: id, MainClass: "net.minecraft.client.main.Main"}, nil
}
func (engine *testEngine) ListJava(context.Context) ([]core.JavaInfo, error) {
	return []core.JavaInfo{{Path: "java.exe", Major: 21}}, nil
}
func (engine *testEngine) DetectJava(_ context.Context, path string) (core.JavaInfo, error) {
	return core.JavaInfo{Path: path, Major: 21}, nil
}

func newTerminalForTest(t *testing.T, engine *testEngine) (*Terminal, *strings.Builder) {
	t.Helper()
	output := &strings.Builder{}
	terminal, err := New(Options{Engine: engine, Config: config.Default(), Version: "test", Output: output})
	if err != nil {
		t.Fatal(err)
	}
	return terminal, output
}

func TestCommandParserSupportsQuotesAndRejectsMalformedInput(t *testing.T) {
	words, err := Parse(`launch --config "folder with spaces\launch.json"`)
	if err != nil {
		t.Fatal(err)
	}
	if len(words) != 3 || words[2] != `folder with spaces\launch.json` {
		t.Fatalf("Parse() = %#v", words)
	}
	if _, err := Parse(`launch --config "unfinished`); err == nil {
		t.Fatal("Parse() accepted an unterminated quote")
	}
	if _, err := Parse("launch trailing\\"); err == nil {
		t.Fatal("Parse() accepted an incomplete escape")
	}
}

func TestRegistryHelpClearVersionAndExit(t *testing.T) {
	engine := newTestEngine()
	terminal, output := newTerminalForTest(t, engine)
	names := terminal.Registry().Names()
	if !contains(names, "launch") || !contains(names, "debug") || !contains(names, "account") || !contains(names, "help") {
		t.Fatalf("command registry is incomplete: %v", names)
	}
	if err := terminal.Registry().Register("launch", func(*Terminal, []string) error { return nil }); err == nil {
		t.Fatal("Registry.Register accepted a duplicate command")
	}
	if _, err := terminal.Execute("unknown"); err == nil {
		t.Fatal("unknown command was accepted")
	}
	if _, err := terminal.Execute("stop"); err == nil {
		t.Fatal("invalid command arguments were accepted")
	}
	for _, line := range []string{"help", "clear", "version"} {
		if _, err := terminal.Execute(line); err != nil {
			t.Fatalf("Execute(%q): %v", line, err)
		}
	}
	if keepGoing, err := terminal.Execute("exit"); err != nil || keepGoing {
		t.Fatalf("exit returned keepGoing=%t err=%v", keepGoing, err)
	}
	if !strings.Contains(output.String(), "Commands:") || !strings.Contains(output.String(), "ChesLaunchEngine test") {
		t.Fatalf("help/version output missing: %s", output.String())
	}
}

func TestLaunchLoadsRequestAndStartsCoreTask(t *testing.T) {
	engine := newTestEngine()
	terminal, output := newTerminalForTest(t, engine)
	requestPath := t.TempDir() + "/request.json"
	requestJSON := `{"versionId":"1.21.1","gameDirectory":".minecraft","account":{"name":"OfflinePlayer","type":"offline"}}`
	if err := os.WriteFile(requestPath, []byte(requestJSON), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := terminal.Execute("launch --config " + strconv.Quote(requestPath)); err != nil {
		t.Fatal(err)
	}
	if engine.startedRequest.VersionID != "1.21.1" {
		t.Fatalf("launch request was not loaded: %+v", engine.startedRequest)
	}
	if !strings.Contains(output.String(), "Launch Task ID: task-1") || !strings.Contains(output.String(), "Minecraft version: 1.21.1") {
		t.Fatalf("launch details missing: %s", output.String())
	}
	if _, err := terminal.Execute("launch --config"); err == nil {
		t.Fatal("launch accepted a missing request path")
	}
}

func TestNonInteractiveLaunchWaitsForTaskCompletion(t *testing.T) {
	engine := newTestEngine()
	engine.finishStatus = models.LaunchExited
	requestPath := t.TempDir() + "/request.json"
	requestJSON := `{"versionId":"1.21.1","gameDirectory":".minecraft","account":{"name":"OfflinePlayer","type":"offline"}}`
	if err := os.WriteFile(requestPath, []byte(requestJSON), 0600); err != nil {
		t.Fatal(err)
	}
	output := &strings.Builder{}
	terminal, err := New(Options{
		Engine: engine, Config: config.Default(), Version: "test", Output: output,
		Input: strings.NewReader("launch --config " + strconv.Quote(requestPath) + "\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := terminal.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "task-1 version=1.21.1 stage=exited") {
		t.Fatalf("final task status was not shown: %s", output.String())
	}
}

func TestOfflineAccountCommandsAndLaunchIntegration(t *testing.T) {
	engine := newTestEngine()
	accountManager, err := accounts.NewManager(t.TempDir() + "/accounts.json")
	if err != nil {
		t.Fatal(err)
	}
	output := &strings.Builder{}
	terminal, err := New(Options{
		Engine: engine, Accounts: accountManager, Config: config.Default(),
		Version: "test", Output: output,
	})
	if err != nil {
		t.Fatal(err)
	}
	customUUID := "12345678-1234-1234-1234-123456789abc"
	if _, err := terminal.Execute("account add offline Test_User " + customUUID); err != nil {
		t.Fatal(err)
	}
	accountList, err := accountManager.ListAccounts(context.Background())
	if err != nil || len(accountList) != 1 {
		t.Fatalf("ListAccounts() = %d, %v", len(accountList), err)
	}
	account := accountList[0]
	if account.UUID != customUUID {
		t.Fatalf("custom UUID = %q", account.UUID)
	}
	for _, command := range []string{
		"account list",
		"account inspect " + account.ID,
		"account profile " + account.ID,
		"account validate " + account.ID,
		"account switch " + account.ID,
	} {
		if _, err := terminal.Execute(command); err != nil {
			t.Fatalf("Execute(%q): %v", command, err)
		}
	}
	for _, command := range []string{"account refresh " + account.ID, "account logout " + account.ID} {
		if _, err := terminal.Execute(command); !errors.Is(err, core.ErrAccountOperationUnsupported) {
			t.Fatalf("Execute(%q) error = %v", command, err)
		}
	}
	requestPath := t.TempDir() + "/request.json"
	requestJSON := `{"versionId":"1.21.1","gameDirectory":".minecraft","account":{"id":"` + account.ID + `"}}`
	if err := os.WriteFile(requestPath, []byte(requestJSON), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := terminal.Execute("launch --config " + strconv.Quote(requestPath)); err != nil {
		t.Fatal(err)
	}
	if engine.startedRequest.Account.ID != account.ID ||
		engine.startedRequest.Account.Name != "" ||
		engine.startedRequest.Account.UUID != "" {
		t.Fatalf("terminal must pass only the account ID to Core: %+v", engine.startedRequest.Account)
	}
	if !strings.Contains(output.String(), customUUID) || strings.Contains(output.String(), "access-secret") {
		t.Fatalf("account output is missing expected data or contains secrets: %s", output.String())
	}
	if _, err := terminal.Execute("account remove " + account.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := accountManager.GetAccount(context.Background(), account.ID); err == nil {
		t.Fatal("account remove command did not delete the account")
	}
}

func TestYggdrasilPasswordRequiresHiddenInteractiveInput(t *testing.T) {
	accountManager, err := accounts.NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	output := &strings.Builder{}
	terminal, err := New(Options{
		Engine: newTestEngine(), Accounts: accountManager, Config: config.Default(),
		Input: strings.NewReader(""), Output: output,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := terminal.Execute("account add yggdrasil https://auth.example.org login"); err == nil ||
		!strings.Contains(err.Error(), "secret input requires an interactive terminal") {
		t.Fatalf("non-interactive Yggdrasil add error = %v", err)
	}
	if _, err := terminal.Execute("account add yggdrasil https://auth.example.org login visible-password"); err == nil ||
		!strings.Contains(err.Error(), "usage: account add yggdrasil") ||
		strings.Contains(err.Error(), "visible-password") {
		t.Fatalf("password command argument was not rejected safely: %v", err)
	}

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	terminal.interactive = true
	terminal.inputFile = reader
	if _, err := writer.Write([]byte("hidden-password\n")); err != nil {
		t.Fatal(err)
	}
	secret, err := terminal.readSecret("Password: ")
	if err != nil {
		t.Fatal(err)
	}
	defer zeroBytes(secret)
	if string(secret) != "hidden-password" || strings.Contains(output.String(), "hidden-password") {
		t.Fatalf("hidden password handling exposed input: secret=%q output=%q", secret, output.String())
	}
}

func TestStatusCancelProcessAndLogCommands(t *testing.T) {
	engine := newTestEngine()
	engine.tasks["task-1"] = models.LaunchTask{ID: "task-1", Status: models.LaunchDownloading, VersionID: "1.21.1"}
	engine.processes = []core.ProcessInfo{{ProcessID: 456, TaskID: "task-1", VersionID: "1.21.1", Status: models.LaunchRunning}}
	engine.events = []core.Event{
		{Source: core.EventDownload, TaskID: "task-1", Message: "Downloaded client"},
		{Source: core.EventGameStderr, TaskID: "task-1", ProcessID: 456, Message: "game diagnostic"},
		{Source: core.EventGameStdout, TaskID: "other", Message: "isolated output"},
	}
	terminal, output := newTerminalForTest(t, engine)
	for _, line := range []string{"status task-1", "tasks", "process list", "process inspect 456", "logs --task task-1", "download status", "download list"} {
		if _, err := terminal.Execute(line); err != nil {
			t.Fatalf("Execute(%q): %v", line, err)
		}
	}
	if _, err := terminal.Execute("cancel task-1"); err != nil {
		t.Fatal(err)
	}
	if len(engine.cancelled) != 1 || engine.cancelled[0] != "task-1" {
		t.Fatalf("cancel calls = %v", engine.cancelled)
	}
	if _, err := terminal.Execute("stop 456"); err != nil {
		t.Fatal(err)
	}
	if len(engine.stopped) != 1 || engine.stopped[0] != 456 {
		t.Fatalf("stop calls = %v", engine.stopped)
	}
	if strings.Contains(output.String(), "isolated output") {
		t.Fatal("task log filtering displayed another task's output")
	}
	if !strings.Contains(output.String(), "game diagnostic") || !strings.Contains(output.String(), "downloads=0/0") {
		t.Fatalf("status/process/log output missing: %s", output.String())
	}
}

func TestVersionsJavaDownloadWatchAndDebugCommands(t *testing.T) {
	engine := newTestEngine()
	engine.tasks["task-1"] = models.LaunchTask{ID: "task-1", Status: models.LaunchRunning, VersionID: "1.21.1"}
	engine.processes = []core.ProcessInfo{{ProcessID: 21, TaskID: "task-1", Status: models.LaunchRunning}}
	engine.events = []core.Event{{Source: core.EventDownload, TaskID: "task-1", Message: "asset progress"}}
	terminal, output := newTerminalForTest(t, engine)
	commands := []string{
		"version list", "version inspect 1.21.1", "version resolve 1.21.1",
		"java list", "java detect", "java inspect java.exe",
		"watch task-1", "unwatch", "logs --process 21 --follow", "logs --task task-1",
		"download cancel task-1", "debug info task-1", "debug config",
		"debug arguments task-1", "debug classpath task-1", "debug environment task-1",
	}
	for _, line := range commands {
		if _, err := terminal.Execute(line); err != nil {
			t.Fatalf("Execute(%q): %v", line, err)
		}
	}
	if !strings.Contains(output.String(), "Java  (major 21") || !strings.Contains(output.String(), "asset progress") {
		t.Fatalf("version/Java/download/debug output missing: %s", output.String())
	}
}

func TestTerminalCtrlCCancelsLaunchAndStopsRunningProcess(t *testing.T) {
	engine := newTestEngine()
	engine.tasks["task-pending"] = models.LaunchTask{ID: "task-pending", Status: models.LaunchDownloading}
	terminal, _ := newTerminalForTest(t, engine)
	terminal.currentTaskID = "task-pending"
	if err := terminal.handleInterrupt(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(engine.cancelled) != 1 {
		t.Fatalf("Ctrl+C did not cancel launch task: %v", engine.cancelled)
	}
	engine.tasks["task-running"] = models.LaunchTask{ID: "task-running", Status: models.LaunchRunning, ProcessID: 42}
	terminal.currentTaskID = "task-running"
	if err := terminal.handleInterrupt(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(engine.stopped) != 1 || engine.stopped[0] != 42 {
		t.Fatalf("Ctrl+C did not gracefully stop running process: %v", engine.stopped)
	}
	if !errors.Is(terminal.handleInterrupt(context.Background()), ErrExit) {
		t.Fatal("Ctrl+C with no active task did not exit")
	}
}

func TestRunNonInteractiveAndUnsubscribesEvents(t *testing.T) {
	engine := newTestEngine()
	output := &strings.Builder{}
	terminal, err := New(Options{
		Engine: engine, Config: config.Default(), Input: strings.NewReader("help\nversion\nexit\n"),
		Output: output,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := terminal.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if engine.unsubscribed != 1 {
		t.Fatalf("event subscription cleanup count = %d", engine.unsubscribed)
	}
	if !strings.Contains(output.String(), "ChesLaunchEngine dev") {
		t.Fatalf("non-interactive command output missing: %s", output.String())
	}
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
