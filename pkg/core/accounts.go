package core

import (
	"context"
	"errors"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

var ErrAccountOperationUnsupported = errors.New("account operation is not supported by this provider")

type AccountProvider interface {
	Type() models.AccountType
	Authenticate(context.Context, models.AccountAuthRequest) (models.Account, error)
	Refresh(context.Context, models.Account) (models.Account, error)
	Validate(context.Context, models.Account) error
	GetProfile(context.Context, models.Account) (models.GameProfile, error)
	Logout(context.Context, models.Account) error
}

type AccountManager interface {
	Provider(models.AccountType) (AccountProvider, error)
	AuthenticateAccount(context.Context, models.AccountAuthRequest) (models.Account, error)
	ListAccounts(context.Context) ([]models.Account, error)
	GetAccount(context.Context, string) (models.Account, error)
	ValidateAccount(context.Context, string) error
	RefreshAccount(context.Context, string) (models.Account, error)
	GetAccountProfile(context.Context, string) (models.GameProfile, error)
	LogoutAccount(context.Context, string) error
	RemoveAccount(context.Context, string) error
	SetActiveAccount(context.Context, string) error
	ActiveAccount(context.Context) (models.Account, error)
}
