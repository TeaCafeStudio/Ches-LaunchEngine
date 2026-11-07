package accounts

import (
	"context"
	"errors"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/core"
	cleerrors "github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/errors"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

func ResolveLaunchAccount(ctx context.Context, manager core.AccountManager, account models.Account) (models.Account, error) {
	hasIdentity := account.Name != "" || account.DisplayName != "" || account.Profile.Name != ""
	if manager == nil {
		if account.ID != "" {
			return models.Account{}, cleerrors.New(cleerrors.CodeAccountNotFound, "launch request references a stored account but no account manager is configured")
		}
		return normalizeLaunchAccount(account), nil
	}
	if account.ID != "" {
		stored, err := manager.GetAccount(ctx, account.ID)
		if err != nil {
			return models.Account{}, cleerrors.Wrap(cleerrors.CodeAccountNotFound, "stored launch account was not found", err)
		}
		if account.Type != "" && account.Type != stored.Type {
			return models.Account{}, cleerrors.New(cleerrors.CodeAccountTypeUnsupported, "launch account type does not match the stored account")
		}
		return resolveStoredAccount(ctx, manager, stored)
	}
	if !hasIdentity && (account.Type == "" || account.Type == models.AccountOffline) {
		stored, err := manager.ActiveAccount(ctx)
		if err != nil {
			return models.Account{}, err
		}
		return resolveStoredAccount(ctx, manager, stored)
	}
	if account.Type != "" && account.Type != models.AccountOffline {
		return models.Account{}, cleerrors.Wrap(cleerrors.CodeAccountTypeUnsupported, "this launch account type requires a supported stored account", core.ErrAccountOperationUnsupported)
	}
	return normalizeLaunchAccount(account), nil
}

type sessionResolver interface {
	ResolveSession(context.Context, models.Account) (models.AccountSession, error)
}

type launchAccountResolver interface {
	ResolveLaunchAccount(context.Context, models.Account) (models.Account, error)
}

type managerLaunchAccountResolver interface {
	ResolveLaunchAccount(context.Context, string) (models.Account, error)
}

func resolveStoredAccount(ctx context.Context, manager core.AccountManager, account models.Account) (models.Account, error) {
	account = normalizeLaunchAccount(account)
	if !requiresSessionResolution(account.Type) {
		return account, nil
	}
	if resolver, ok := manager.(managerLaunchAccountResolver); ok {
		return resolver.ResolveLaunchAccount(ctx, account.ID)
	}
	return resolveStoredAccountWithProvider(ctx, manager, account)
}

func requiresSessionResolution(accountType models.AccountType) bool {
	return accountType == models.AccountYggdrasil || accountType == models.AccountMicrosoft
}

func resolveStoredAccountWithProvider(ctx context.Context, manager core.AccountManager, account models.Account) (models.Account, error) {
	provider, err := manager.Provider(account.Type)
	if err != nil {
		return models.Account{}, cleerrors.Wrap(cleerrors.CodeAccountTypeUnsupported, "account provider is unavailable", errors.Join(core.ErrAccountOperationUnsupported, err))
	}
	if resolver, ok := provider.(launchAccountResolver); ok {
		resolved, err := resolver.ResolveLaunchAccount(ctx, account)
		if err != nil {
			return models.Account{}, err
		}
		return resolved, nil
	}
	resolver, ok := provider.(sessionResolver)
	if !ok {
		return models.Account{}, cleerrors.New(cleerrors.CodeAccountTypeUnsupported, "account provider cannot resolve an authenticated session")
	}
	account.Session, err = resolver.ResolveSession(ctx, account)
	if err != nil {
		return models.Account{}, err
	}
	return account, nil
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
