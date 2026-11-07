package accounts

import (
	"context"
	"crypto/md5"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/core"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

var (
	usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,16}$`)
	uuidPattern     = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

type OfflineProvider struct{}

func NewOfflineProvider() OfflineProvider {
	return OfflineProvider{}
}

func (OfflineProvider) Type() models.AccountType {
	return models.AccountOffline
}

func (provider OfflineProvider) Authenticate(ctx context.Context, request models.AccountAuthRequest) (models.Account, error) {
	if err := ctx.Err(); err != nil {
		return models.Account{}, err
	}
	if request.Type != models.AccountOffline {
		return models.Account{}, fmt.Errorf("%w: offline provider cannot authenticate %q accounts", core.ErrAccountOperationUnsupported, request.Type)
	}
	username := strings.TrimSpace(request.Username)
	if err := ValidateOfflineIdentity(username, request.UUID); err != nil {
		return models.Account{}, err
	}
	uuid := strings.ToLower(strings.TrimSpace(request.UUID))
	if uuid == "" {
		uuid = OfflineUUID(username)
	}
	id, err := newAccountID()
	if err != nil {
		return models.Account{}, err
	}
	now := time.Now().UTC()
	return models.Account{
		ID:          id,
		Name:        username,
		UUID:        uuid,
		DisplayName: username,
		Profile:     models.GameProfile{ID: uuid, Name: username},
		Type:        models.AccountOffline, UserType: "legacy",
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (OfflineProvider) Refresh(context.Context, models.Account) (models.Account, error) {
	return models.Account{}, fmt.Errorf("%w: offline accounts have no expiring session", core.ErrAccountOperationUnsupported)
}

func (provider OfflineProvider) Validate(ctx context.Context, account models.Account) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if account.Type != models.AccountOffline {
		return fmt.Errorf("%w: offline provider cannot validate %q accounts", core.ErrAccountOperationUnsupported, account.Type)
	}
	if !usernamePattern.MatchString(account.Name) {
		return errorsInvalidOfflineName()
	}
	if !uuidPattern.MatchString(account.UUID) || account.Profile.ID != account.UUID || account.Profile.Name != account.Name {
		return fmt.Errorf("offline account profile is invalid")
	}
	return nil
}

func ValidateOfflineIdentity(username, uuid string) error {
	if !usernamePattern.MatchString(username) {
		return errorsInvalidOfflineName()
	}
	if uuid != "" && !uuidPattern.MatchString(uuid) {
		return fmt.Errorf("offline account UUID must use the 8-4-4-4-12 hexadecimal format")
	}
	return nil
}

func (provider OfflineProvider) GetProfile(ctx context.Context, account models.Account) (models.GameProfile, error) {
	if err := provider.Validate(ctx, account); err != nil {
		return models.GameProfile{}, err
	}
	return cloneProfile(account.Profile), nil
}

func (OfflineProvider) Logout(context.Context, models.Account) error {
	return fmt.Errorf("%w: offline accounts have no authentication session", core.ErrAccountOperationUnsupported)
}

func OfflineUUID(username string) string {
	sum := md5.Sum([]byte("OfflinePlayer:" + username))
	sum[6] = (sum[6] & 0x0f) | 0x30
	sum[8] = (sum[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}

func errorsInvalidOfflineName() error {
	return fmt.Errorf("offline account name must contain 1 to 16 letters, digits, or underscores")
}
