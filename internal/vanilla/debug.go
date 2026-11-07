package vanilla

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/download"
	javaruntime "github.com/TeaCafeStudio/Ches-LaunchEngine/internal/runtime"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/core"
	cleerrors "github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/errors"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

const eventHistoryLimit = 2000
const maxEventSubscribers = 32
const maxEventBuffer = 512
const maxEventMessageSize = 16 << 10
const maxEventDataValueSize = 4 << 10

var _ core.DebugEngine = (*Engine)(nil)

func (engine *Engine) emit(event core.Event) {
	engine.mu.Lock()
	engine.emitLocked(event)
	engine.mu.Unlock()
}

func (engine *Engine) emitLocked(event core.Event) {
	event.EventID = strconv.FormatUint(engine.nextEventID.Add(1), 10)
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	event.Message = core.RedactText(event.Message)
	event.Data = cloneStrings(event.Data)
	if len(event.Message) > maxEventMessageSize {
		event.Message = strings.ToValidUTF8(event.Message[:maxEventMessageSize], "\uFFFD") + "...[truncated]"
	}
	for key, value := range event.Data {
		if len(value) > maxEventDataValueSize {
			event.Data[key] = strings.ToValidUTF8(value[:maxEventDataValueSize], "\uFFFD") + "...[truncated]"
		}
	}
	engine.eventLog = append(engine.eventLog, event)
	if len(engine.eventLog) > eventHistoryLimit {
		copy(engine.eventLog, engine.eventLog[len(engine.eventLog)-eventHistoryLimit:])
		engine.eventLog = engine.eventLog[:eventHistoryLimit]
	}
	for _, subscriber := range engine.eventSubs {
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
}

func cloneStrings(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	result := core.RedactEnvironment(values)
	for key, value := range result {
		if len(value) > maxEventDataValueSize {
			result[key] = strings.ToValidUTF8(value[:maxEventDataValueSize], "\uFFFD") + "...[truncated]"
		}
	}
	return result
}

func (engine *Engine) reportDownload(taskID string, progress download.Progress) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	state := engine.tasks[taskID]
	if state == nil {
		return
	}
	if state.downloads == nil {
		state.downloads = make(map[string]bool)
	}
	key := progress.Path
	completed, known := state.downloads[key]
	if !known {
		state.downloads[key] = false
		state.debug.DownloadsTotal++
	}
	if progress.Completed && !completed {
		state.downloads[key] = true
		state.debug.DownloadsDone++
	}
	message := "Downloading " + filepath.Base(progress.Path)
	if progress.Completed {
		message = "Downloaded " + filepath.Base(progress.Path)
	}
	engine.emitLocked(core.Event{
		Source: core.EventDownload, TaskID: taskID, ProcessID: state.task.ProcessID,
		Level: "INFO", Message: message,
		Data: map[string]string{
			"path": filepath.Base(progress.Path), "receivedBytes": strconv.FormatInt(progress.Received, 10),
			"totalBytes": strconv.FormatInt(progress.Total, 10),
			"completed":  strconv.FormatBool(progress.Completed),
			"cached":     strconv.FormatBool(progress.Cached),
			"filesDone":  strconv.Itoa(state.debug.DownloadsDone),
			"filesTotal": strconv.Itoa(state.debug.DownloadsTotal),
		},
	})
}

func (engine *Engine) SubscribeEvents(buffer int) (<-chan core.Event, func(), error) {
	if buffer < 1 {
		buffer = 128
	}
	if buffer > maxEventBuffer {
		return nil, nil, fmt.Errorf("event subscription buffer must not exceed %d", maxEventBuffer)
	}
	engine.mu.Lock()
	if engine.closed {
		engine.mu.Unlock()
		return nil, nil, errors.New("Minecraft engine is closed")
	}
	if len(engine.eventSubs) >= maxEventSubscribers {
		engine.mu.Unlock()
		return nil, nil, fmt.Errorf("event subscriber limit (%d) reached", maxEventSubscribers)
	}
	id := engine.nextSubID.Add(1)
	events := make(chan core.Event, buffer)
	engine.eventSubs[id] = events
	engine.mu.Unlock()
	unsubscribe := func() {
		engine.mu.Lock()
		if current, ok := engine.eventSubs[id]; ok {
			delete(engine.eventSubs, id)
			close(current)
		}
		engine.mu.Unlock()
	}
	return events, unsubscribe, nil
}

func (engine *Engine) RecentEvents(filter core.EventFilter, limit int) []core.Event {
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	if limit <= 0 || limit > eventHistoryLimit {
		limit = eventHistoryLimit
	}
	matches := make([]core.Event, 0, limit)
	for index := len(engine.eventLog) - 1; index >= 0 && len(matches) < limit; index-- {
		event := engine.eventLog[index]
		if filter.TaskID != "" && event.TaskID != filter.TaskID {
			continue
		}
		if filter.ProcessID != 0 && event.ProcessID != filter.ProcessID {
			continue
		}
		event.Data = cloneStrings(event.Data)
		matches = append(matches, event)
	}
	for left, right := 0, len(matches)-1; left < right; left, right = left+1, right-1 {
		matches[left], matches[right] = matches[right], matches[left]
	}
	return matches
}

func (engine *Engine) ListLaunchTasks() []models.LaunchTask {
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	tasks := make([]models.LaunchTask, 0, len(engine.tasks))
	for _, state := range engine.tasks {
		tasks = append(tasks, cloneTask(state.task))
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].StartedAt.Before(tasks[j].StartedAt) })
	return tasks
}

func cloneTask(task models.LaunchTask) models.LaunchTask {
	if task.ExitCode != nil {
		code := *task.ExitCode
		task.ExitCode = &code
	}
	return task
}

func (engine *Engine) ListProcesses() []core.ProcessInfo {
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	processes := make([]core.ProcessInfo, 0, len(engine.tasks))
	for _, state := range engine.tasks {
		if state.task.ProcessID == 0 {
			continue
		}
		processes = append(processes, processInfo(state.task))
	}
	sort.Slice(processes, func(i, j int) bool { return processes[i].ProcessID < processes[j].ProcessID })
	return processes
}

func processInfo(task models.LaunchTask) core.ProcessInfo {
	return core.ProcessInfo{
		ProcessID: task.ProcessID, TaskID: task.ID, VersionID: task.VersionID,
		Status: task.Status, StartedAt: task.StartedAt,
		ExitCode: cloneTask(task).ExitCode,
	}
}

func (engine *Engine) InspectProcess(processID int) (core.ProcessInfo, error) {
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	for _, state := range engine.tasks {
		if state.task.ProcessID == processID {
			return processInfo(state.task), nil
		}
	}
	return core.ProcessInfo{}, cleerrors.New(cleerrors.CodeProcessNotFound, "process is not managed by this engine")
}

func (engine *Engine) StopProcess(ctx context.Context, processID int, force bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	engine.mu.Lock()
	var state *taskState
	for _, candidate := range engine.tasks {
		if candidate.task.ProcessID == processID && candidate.proc != nil {
			state = candidate
			break
		}
	}
	if state == nil {
		engine.mu.Unlock()
		return cleerrors.New(cleerrors.CodeProcessNotFound, "running process is not managed by this engine")
	}
	process := state.proc
	state.stopped = true
	engine.mu.Unlock()

	var err error
	if force {
		err = process.Kill()
	} else {
		err = stopProcess(process)
	}
	if err != nil && !errors.Is(err, os.ErrProcessDone) {
		engine.mu.Lock()
		state.stopped = false
		engine.mu.Unlock()
		return cleerrors.Wrap(cleerrors.CodeProcessStartFailed, "stop Minecraft process", err)
	}
	action := "graceful stop requested"
	if force {
		action = "force stop requested"
	}
	engine.emit(core.Event{
		Source: core.EventEngineLog, TaskID: state.task.ID, ProcessID: processID,
		Level: "WARN", Message: "Minecraft process " + action,
	})
	return nil
}

func (engine *Engine) CancelDownloads(_ context.Context, taskID string) error {
	engine.mu.RLock()
	state, ok := engine.tasks[taskID]
	if !ok {
		engine.mu.RUnlock()
		return cleerrors.New(cleerrors.CodeTaskNotFound, "launch task was not found")
	}
	if state.task.Status != models.LaunchDownloading || state.cancel == nil {
		engine.mu.RUnlock()
		return errors.New("task has no active download operation")
	}
	cancel := state.cancel
	engine.mu.RUnlock()
	cancel()
	return nil
}

func (engine *Engine) GetLaunchDebugInfo(taskID string) (core.LaunchDebugInfo, error) {
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	state, ok := engine.tasks[taskID]
	if !ok {
		return core.LaunchDebugInfo{}, cleerrors.New(cleerrors.CodeTaskNotFound, "launch task was not found")
	}
	info := state.debug
	info.Task = cloneTask(state.task)
	info.Stage = state.task.Status
	info.JVMArguments = append([]string(nil), info.JVMArguments...)
	info.GameArguments = append([]string(nil), info.GameArguments...)
	info.Classpath = append([]string(nil), info.Classpath...)
	info.Environment = cloneStrings(info.Environment)
	return info, nil
}

func (engine *Engine) ListVersions(ctx context.Context) ([]core.VersionInfo, error) {
	gameDirectory, err := engine.metadataGameDirectory()
	if err != nil {
		return nil, err
	}
	manifest, err := engine.resolver.ListVersions(ctx, gameDirectory, engine.config.Download.Offline)
	if err != nil {
		return nil, err
	}
	result := make([]core.VersionInfo, 0, len(manifest.Versions))
	for _, entry := range manifest.Versions {
		release, _ := time.Parse(time.RFC3339, entry.ReleaseTime)
		result = append(result, core.VersionInfo{ID: entry.ID, Type: entry.Type, URL: entry.URL, ReleaseTime: release})
	}
	return result, nil
}

func (engine *Engine) InspectVersion(ctx context.Context, versionID string) (core.VersionInfo, error) {
	gameDirectory, err := engine.metadataGameDirectory()
	if err != nil {
		return core.VersionInfo{}, err
	}
	version, err := engine.resolver.Resolve(ctx, gameDirectory, versionID, engine.config.Download.Offline)
	if err != nil {
		return core.VersionInfo{}, err
	}
	info := core.VersionInfo{
		ID: version.ID, Type: version.Type, MainClass: version.MainClass,
		AssetIndex: version.Assets, LibraryCount: len(version.Libraries),
	}
	if version.JavaVersion != nil {
		info.JavaMajor = version.JavaVersion.MajorVersion
	}
	info.ReleaseTime, _ = time.Parse(time.RFC3339, version.ReleaseTime)
	return info, nil
}

func (engine *Engine) metadataGameDirectory() (string, error) {
	directory := engine.config.Paths.GameDirectory
	if directory == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve default game directory: %w", err)
		}
		directory = filepath.Join(home, ".minecraft")
	}
	directory, err := filepath.Abs(directory)
	if err != nil {
		return "", fmt.Errorf("resolve game directory: %w", err)
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		return "", fmt.Errorf("create game directory: %w", err)
	}
	return directory, nil
}

func (engine *Engine) ListJava(ctx context.Context) ([]core.JavaInfo, error) {
	candidates := make(map[string]struct{})
	add := func(path string) {
		if path == "" {
			return
		}
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			name := "java"
			if runtime.GOOS == "windows" {
				name = "java.exe"
			}
			path = filepath.Join(path, "bin", name)
		}
		absolute, err := filepath.Abs(path)
		if err == nil {
			candidates[filepath.Clean(absolute)] = struct{}{}
		}
	}
	add(engine.config.Java.Path)
	if java, err := exec.LookPath("java"); err == nil {
		add(java)
	}
	executable := "java"
	if runtime.GOOS == "windows" {
		executable = "java.exe"
	}
	for _, directory := range filepath.SplitList(os.Getenv("PATH")) {
		add(filepath.Join(directory, executable))
	}
	if javaHome := os.Getenv("JAVA_HOME"); javaHome != "" {
		add(javaHome)
	}
	paths := make([]string, 0, len(candidates))
	for path := range candidates {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	result := make([]core.JavaInfo, 0, len(paths))
	for _, path := range paths {
		info, err := engine.DetectJava(ctx, path)
		if err == nil {
			result = append(result, info)
		} else if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return result, nil
}

func (engine *Engine) DetectJava(ctx context.Context, path string) (core.JavaInfo, error) {
	if path == "" {
		path = engine.config.Java.Path
	}
	detectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	info, err := javaruntime.DetectJava(detectCtx, path)
	if err != nil {
		return core.JavaInfo{}, err
	}
	return core.JavaInfo{
		Path: info.Path, Version: info.Version, Major: info.Major, Architecture: info.Architecture,
	}, nil
}
