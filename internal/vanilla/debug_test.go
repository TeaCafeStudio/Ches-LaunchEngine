package vanilla

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/config"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/download"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/core"
	cleerrors "github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/errors"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

func newDebugTestEngine(t *testing.T) *Engine {
	t.Helper()
	engine, err := NewEngine(EngineOptions{
		Config: config.Default(),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Errorf("close engine: %v", err)
		}
	})
	return engine
}

func TestEventSubscriptionIsBoundedAndUnsubscribes(t *testing.T) {
	engine := newDebugTestEngine(t)
	if _, _, err := engine.SubscribeEvents(maxEventBuffer + 1); err == nil {
		t.Fatal("event subscription accepted an unbounded buffer")
	}
	events, unsubscribe, err := engine.SubscribeEvents(1)
	if err != nil {
		t.Fatal(err)
	}
	engine.emit(core.Event{Source: core.EventEngineLog, Message: "first"})
	engine.emit(core.Event{Source: core.EventEngineLog, Message: "second"})
	event := <-events
	if event.Message != "second" {
		t.Fatalf("overflow retained %q, want newest event", event.Message)
	}
	unsubscribe()
	if _, open := <-events; open {
		t.Fatal("unsubscribed event channel remains open")
	}
}

func TestEventHistoryBoundsMessageSize(t *testing.T) {
	engine := newDebugTestEngine(t)
	engine.emit(core.Event{Source: core.EventEngineLog, Message: strings.Repeat("x", maxEventMessageSize+10)})
	events := engine.RecentEvents(core.EventFilter{}, 1)
	if len(events) != 1 || len(events[0].Message) > maxEventMessageSize+32 ||
		!strings.HasSuffix(events[0].Message, "...[truncated]") {
		t.Fatalf("oversized event was not bounded: %d bytes", len(events[0].Message))
	}
}

func TestCloseReleasesEventSubscribers(t *testing.T) {
	engine, err := NewEngine(EngineOptions{Config: config.Default()})
	if err != nil {
		t.Fatal(err)
	}
	events, _, err := engine.SubscribeEvents(1)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	if _, open := <-events; open {
		t.Fatal("engine close did not close the event subscription")
	}
}

func TestTaskStatusChangesPublishCoreEventsWithoutSubscribers(t *testing.T) {
	engine := newDebugTestEngine(t)
	engine.tasks["task-a"] = &taskState{task: models.LaunchTask{
		ID: "task-a", Status: models.LaunchResolving,
	}}
	engine.updateTask("task-a", models.LaunchFailed, cleerrors.New(cleerrors.CodeJavaNotFound, "Java not found"))
	events := engine.RecentEvents(core.EventFilter{TaskID: "task-a"}, 10)
	if len(events) != 1 || events[0].Source != core.EventTaskStatus ||
		events[0].Data["errorCode"] != string(cleerrors.CodeJavaNotFound) {
		t.Fatalf("task status event = %+v", events)
	}
	task, err := engine.GetLaunchStatus("task-a")
	if err != nil {
		t.Fatal(err)
	}
	if task.FailedAtStage != models.LaunchResolving {
		t.Fatalf("failure stage = %s", task.FailedAtStage)
	}
}

func TestStartLaunchReturnsTaskAndPublishesAsyncFailure(t *testing.T) {
	engine := newDebugTestEngine(t)
	task, err := engine.StartLaunch(context.Background(), models.LaunchRequest{})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for task.Status != models.LaunchFailed && time.Now().Before(deadline) {
		task, err = engine.GetLaunchStatus(task.ID)
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
	if task.Status != models.LaunchFailed || task.FailedAtStage != models.LaunchResolving ||
		task.ErrorCode != string(cleerrors.CodeVersionJSONInvalid) {
		t.Fatalf("asynchronous launch failure = %+v", task)
	}
}

func TestGameStdoutStderrAndTaskIsolation(t *testing.T) {
	engine := newDebugTestEngine(t)
	engine.tasks["task-a"] = &taskState{task: models.LaunchTask{ID: "task-a", ProcessID: 101}}
	engine.tasks["task-b"] = &taskState{task: models.LaunchTask{ID: "task-b", ProcessID: 202}}
	engine.captureOutput("task-a", "stdout", strings.NewReader("stdout accessToken=do-not-show\n"))
	engine.captureOutput("task-a", "stderr", strings.NewReader("stderr line\n"))
	engine.captureOutput("task-b", "stdout", strings.NewReader("other task\n"))

	a := engine.RecentEvents(core.EventFilter{TaskID: "task-a"}, 10)
	if len(a) != 2 || a[0].Source != core.EventGameStdout || a[1].Source != core.EventGameStderr {
		t.Fatalf("task A output events = %+v", a)
	}
	if a[0].ProcessID != 101 || a[1].ProcessID != 101 {
		t.Fatalf("task A process IDs = %+v", a)
	}
	if strings.Contains(a[0].Message, "do-not-show") || !strings.Contains(a[0].Message, "[REDACTED]") {
		t.Fatalf("stdout token was not redacted: %q", a[0].Message)
	}
	if got := engine.RecentEvents(core.EventFilter{ProcessID: 202}, 10); len(got) != 1 || got[0].TaskID != "task-b" {
		t.Fatalf("task B process filter = %+v", got)
	}
}

func TestDownloadProgressEventsAndTaskState(t *testing.T) {
	engine := newDebugTestEngine(t)
	engine.tasks["task-a"] = &taskState{
		task:  models.LaunchTask{ID: "task-a", Status: models.LaunchDownloading},
		debug: core.LaunchDebugInfo{},
	}
	engine.reportDownload("task-a", download.Progress{Path: "assets\\abc", Received: 5, Total: 10})
	engine.reportDownload("task-a", download.Progress{Path: "assets\\abc", Received: 10, Total: 10, Completed: true})
	info, err := engine.GetLaunchDebugInfo("task-a")
	if err != nil {
		t.Fatal(err)
	}
	if info.DownloadsDone != 1 || info.DownloadsTotal != 1 {
		t.Fatalf("download counts = %d/%d", info.DownloadsDone, info.DownloadsTotal)
	}
	events := engine.RecentEvents(core.EventFilter{TaskID: "task-a"}, 10)
	if len(events) != 2 || events[1].Source != core.EventDownload || events[1].Data["completed"] != "true" {
		t.Fatalf("download events = %+v", events)
	}
}

func TestLaunchTasksAndManagedProcessSnapshots(t *testing.T) {
	engine := newDebugTestEngine(t)
	exitCode := 0
	engine.tasks["finished"] = &taskState{task: models.LaunchTask{
		ID: "finished", Status: models.LaunchExited, ProcessID: 345, VersionID: "1.21.1", ExitCode: &exitCode,
	}}
	tasks := engine.ListLaunchTasks()
	if len(tasks) != 1 || tasks[0].ID != "finished" {
		t.Fatalf("launch tasks = %+v", tasks)
	}
	processes := engine.ListProcesses()
	if len(processes) != 1 || processes[0].ProcessID != 345 || processes[0].ExitCode == nil {
		t.Fatalf("process list = %+v", processes)
	}
	if _, err := engine.InspectProcess(345); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.InspectProcess(999); err == nil {
		t.Fatal("InspectProcess accepted an unknown PID")
	}
}

func TestCancelTaskDoesNotStopRunningProcess(t *testing.T) {
	engine := newDebugTestEngine(t)
	process := &os.Process{Pid: os.Getpid()}
	engine.tasks["task-running"] = &taskState{
		task: models.LaunchTask{ID: "task-running", Status: models.LaunchRunning, ProcessID: process.Pid},
		proc: process,
	}
	err := engine.CancelTask(context.Background(), "task-running")
	if err == nil || !strings.Contains(err.Error(), "stop the process") {
		t.Fatalf("CancelTask() error = %v", err)
	}
	engine.mu.Lock()
	engine.closed = true
	engine.mu.Unlock()
}
