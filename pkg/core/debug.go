package core

import (
	"context"
	"time"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

type EventSource string

const (
	EventEngineLog      EventSource = "engine_log"
	EventGameStdout     EventSource = "game_stdout"
	EventGameStderr     EventSource = "game_stderr"
	EventDownload       EventSource = "download_progress"
	EventTaskStatus     EventSource = "task_status"
	EventProcessStarted EventSource = "process_started"
	EventProcessExited  EventSource = "process_exited"
)

type Event struct {
	EventID   string            `json:"eventId"`
	Timestamp time.Time         `json:"timestamp"`
	Source    EventSource       `json:"source"`
	TaskID    string            `json:"taskId,omitempty"`
	ProcessID int               `json:"processId,omitempty"`
	Level     string            `json:"level,omitempty"`
	Message   string            `json:"message"`
	Data      map[string]string `json:"data,omitempty"`
}

type EventFilter struct {
	TaskID    string
	ProcessID int
}

type JavaInfo struct {
	Path         string `json:"path"`
	Version      string `json:"version,omitempty"`
	Major        int    `json:"major,omitempty"`
	Architecture string `json:"architecture,omitempty"`
}

type VersionInfo struct {
	ID           string    `json:"id"`
	Type         string    `json:"type,omitempty"`
	URL          string    `json:"url,omitempty"`
	ReleaseTime  time.Time `json:"releaseTime,omitempty"`
	JavaMajor    int       `json:"javaMajor,omitempty"`
	MainClass    string    `json:"mainClass,omitempty"`
	AssetIndex   string    `json:"assetIndex,omitempty"`
	LibraryCount int       `json:"libraryCount,omitempty"`
}

type ProcessInfo struct {
	ProcessID int                 `json:"processId"`
	TaskID    string              `json:"taskId"`
	VersionID string              `json:"versionId"`
	Status    models.LaunchStatus `json:"status"`
	StartedAt time.Time           `json:"startedAt,omitempty"`
	ExitCode  *int                `json:"exitCode,omitempty"`
}

type LaunchDebugInfo struct {
	Task             models.LaunchTask   `json:"task"`
	VersionID        string              `json:"versionId"`
	JavaPath         string              `json:"javaPath,omitempty"`
	AccountType      models.AccountType  `json:"accountType,omitempty"`
	GameDirectory    string              `json:"gameDirectory,omitempty"`
	LoaderType       models.LoaderType   `json:"loaderType,omitempty"`
	Stage            models.LaunchStatus `json:"stage"`
	JVMArguments     []string            `json:"jvmArguments,omitempty"`
	GameArguments    []string            `json:"gameArguments,omitempty"`
	MainClass        string              `json:"mainClass,omitempty"`
	Classpath        []string            `json:"classpath,omitempty"`
	WorkingDirectory string              `json:"workingDirectory,omitempty"`
	Environment      map[string]string   `json:"environment,omitempty"`
	DownloadsDone    int                 `json:"downloadsDone"`
	DownloadsTotal   int                 `json:"downloadsTotal"`
}

type DebugEngine interface {
	MinecraftEngine
	StartLaunch(context.Context, models.LaunchRequest) (models.LaunchTask, error)
	SubscribeEvents(buffer int) (<-chan Event, func(), error)
	RecentEvents(EventFilter, int) []Event
	ListLaunchTasks() []models.LaunchTask
	ListProcesses() []ProcessInfo
	InspectProcess(int) (ProcessInfo, error)
	StopProcess(context.Context, int, bool) error
	CancelDownloads(context.Context, string) error
	GetLaunchDebugInfo(string) (LaunchDebugInfo, error)
	ListVersions(context.Context) ([]VersionInfo, error)
	InspectVersion(context.Context, string) (VersionInfo, error)
	ListJava(context.Context) ([]JavaInfo, error)
	DetectJava(context.Context, string) (JavaInfo, error)
}
