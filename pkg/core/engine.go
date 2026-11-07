package core

import (
	"context"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

// MinecraftEngine is the shared contract used by future CLI and API adapters.
// Implementations must keep launch tasks independent and honor context
// cancellation for long-running operations.
type MinecraftEngine interface {
	Validate(ctx context.Context, request models.LaunchRequest) error
	Launch(ctx context.Context, request models.LaunchRequest) (models.LaunchTask, error)
	GetLaunchStatus(taskID string) (models.LaunchTask, error)
	SubscribeTask(taskID string) (<-chan models.LaunchStatusEvent, func(), error)
	CancelTask(ctx context.Context, taskID string) error
	Close() error
}
