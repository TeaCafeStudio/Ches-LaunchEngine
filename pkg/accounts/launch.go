package accounts

import (
	"context"
	"errors"
	"fmt"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/core"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

func ResolveLaunchAccount(ctx context.Context, manager core.AccountManager, account models.Account) (models.Account, error) {
	hasIdentity := account.Name != "" || account.DisplayName != "" || account.Profile.Name != ""
	if manager == nil {
		if account.ID != "" {
			return models.Account{}, errors.New("launch request references a stored account but no account manager is configured")
		}
		return normalizeLaunchAccount(account), nil
	}
	if account.ID != "" {
		stored, err := manager.GetAccount(ctx, account.ID)
		if err != nil {
			return models.Account{}, err
		}
		if account.Type != "" && account.Type != stored.Type {
			return models.Account{}, fmt.Errorf("launch account type %q does not match stored account type %q", account.Type, stored.Type)
		}
		return normalizeLaunchAccount(stored), nil
	}
	if !hasIdentity && (account.Type == "" || account.Type == models.AccountOffline) {
		stored, err := manager.ActiveAccount(ctx)
		if err != nil {
			return models.Account{}, err
		}
		return normalizeLaunchAccount(stored), nil
	}
	if account.Type != "" && account.Type != models.AccountOffline {
		return models.Account{}, fmt.Errorf("%w: %q", core.ErrAccountOperationUnsupported, account.Type)
	}
	return normalizeLaunchAccount(account), nil
}

func normalizeLaunchAccount(account models.Account) models.Account {
	if account.Name == "" {
		account.Name = account.DisplayName
	}
	if account.Name == "" {
		account.Name = account.Profile.Name
	}
	if account.UUID == "" {
		account.UUID = account.Profile.ID
	}
	if account.Type == "" {
		account.Type = models.AccountOffline
	}
	return account
}
