package models

import "time"

type AccountType string

const (
	AccountOffline   AccountType = "offline"
	AccountMicrosoft AccountType = "microsoft"
	AccountYggdrasil AccountType = "yggdrasil"
)

type Account struct {
	ID            string      `json:"id,omitempty"`
	Name          string      `json:"name,omitempty"`
	UUID          string      `json:"uuid,omitempty"`
	DisplayName   string      `json:"displayName,omitempty"`
	Profile       GameProfile `json:"profile,omitempty"`
	Type          AccountType `json:"type,omitempty"`
	CredentialRef string      `json:"credentialRef,omitempty"`
	XUID          string      `json:"xuid,omitempty"`
	UserType      string      `json:"userType,omitempty"`
	CreatedAt     time.Time   `json:"createdAt,omitempty"`
	UpdatedAt     time.Time   `json:"updatedAt,omitempty"`
}

type GameProfile struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Properties []ProfileProperty `json:"properties,omitempty"`
}

type ProfileProperty struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Signature string `json:"signature,omitempty"`
}

type AccountAuthRequest struct {
	Type     AccountType `json:"type"`
	Username string      `json:"username"`
	UUID     string      `json:"uuid,omitempty"`
	Password string      `json:"-"`
}

type AccountSession struct {
	AccessToken  string    `json:"-"`
	RefreshToken string    `json:"-"`
	ClientID     string    `json:"-"`
	ExpiresAt    time.Time `json:"-"`
}

type LoaderType string

const (
	LoaderVanilla  LoaderType = "vanilla"
	LoaderForge    LoaderType = "forge"
	LoaderNeoForge LoaderType = "neoforge"
	LoaderFabric   LoaderType = "fabric"
	LoaderQuilt    LoaderType = "quilt"
)

type Loader struct {
	Type    LoaderType `json:"type,omitempty"`
	Version string     `json:"version,omitempty"`
}

type DownloadOptions struct {
	Offline     bool `json:"offline,omitempty"`
	Concurrency int  `json:"concurrency,omitempty"`
	Retries     int  `json:"retries,omitempty"`
}

type WindowOptions struct {
	Width      int  `json:"width,omitempty"`
	Height     int  `json:"height,omitempty"`
	Fullscreen bool `json:"fullscreen,omitempty"`
}

type ArgumentMode string

const (
	ArgumentModeAuto   ArgumentMode = "auto"
	ArgumentModeLegacy ArgumentMode = "legacy"
	ArgumentModeModern ArgumentMode = "modern"
)

// LaunchRequest distinguishes the version's metadata directory from the
// game's working directory.
type LaunchRequest struct {
	VersionID        string            `json:"versionId"`
	VersionDirectory string            `json:"versionDirectory,omitempty"`
	GameDirectory    string            `json:"gameDirectory"`
	JavaPath         string            `json:"javaPath,omitempty"`
	JavaArguments    []string          `json:"javaArguments,omitempty"`
	GameArguments    []string          `json:"gameArguments,omitempty"`
	Account          Account           `json:"account,omitempty"`
	Loader           Loader            `json:"loader,omitempty"`
	DownloadOptions  DownloadOptions   `json:"downloadOptions,omitempty"`
	Window           WindowOptions     `json:"window,omitempty"`
	Environment      map[string]string `json:"environment,omitempty"`
	WorkingDirectory string            `json:"workingDirectory,omitempty"`
	ArgumentMode     ArgumentMode      `json:"argumentMode,omitempty"`
}

type LaunchStatus string

const (
	LaunchIdle        LaunchStatus = "idle"
	LaunchResolving   LaunchStatus = "resolving"
	LaunchValidating  LaunchStatus = "validating"
	LaunchDownloading LaunchStatus = "downloading"
	LaunchPreparing   LaunchStatus = "preparing"
	LaunchLaunching   LaunchStatus = "launching"
	LaunchRunning     LaunchStatus = "running"
	LaunchExited      LaunchStatus = "exited"
	LaunchStopped     LaunchStatus = "stopped"
	LaunchFailed      LaunchStatus = "failed"
	LaunchCancelled   LaunchStatus = "cancelled"
)

type LaunchTask struct {
	ID            string       `json:"id"`
	Status        LaunchStatus `json:"status"`
	StartedAt     time.Time    `json:"startedAt,omitempty"`
	FinishedAt    time.Time    `json:"finishedAt,omitempty"`
	ProcessID     int          `json:"processId,omitempty"`
	ExitCode      *int         `json:"exitCode,omitempty"`
	ErrorCode     string       `json:"errorCode,omitempty"`
	Error         string       `json:"error,omitempty"`
	FailedAtStage LaunchStatus `json:"failedAtStage,omitempty"`
	VersionID     string       `json:"versionId,omitempty"`
	JavaPath      string       `json:"javaPath,omitempty"`
	AccountType   AccountType  `json:"accountType,omitempty"`
	GameDirectory string       `json:"gameDirectory,omitempty"`
	LoaderType    LoaderType   `json:"loaderType,omitempty"`
}

type LaunchStatusEvent struct {
	TaskID    string       `json:"taskId"`
	Status    LaunchStatus `json:"status"`
	Timestamp time.Time    `json:"timestamp"`
	Message   string       `json:"message,omitempty"`
}
