package accounts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/core"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

const accountStoreVersion = 1
const maximumAccountStoreSize = 4 << 20

type accountStore struct {
	Version  int              `json:"version"`
	ActiveID string           `json:"activeAccountId,omitempty"`
	Accounts []models.Account `json:"accounts"`
}

type Manager struct {
	mu        sync.RWMutex
	path      string
	providers map[models.AccountType]core.AccountProvider
	accounts  map[string]models.Account
	activeID  string
}

func NewManager(path string, providers ...core.AccountProvider) (*Manager, error) {
	manager := &Manager{
		path: path, providers: make(map[models.AccountType]core.AccountProvider),
		accounts: make(map[string]models.Account),
	}
	if err := manager.RegisterProvider(NewOfflineProvider()); err != nil {
		return nil, err
	}
	for _, provider := range providers {
		if err := manager.RegisterProvider(provider); err != nil {
			return nil, err
		}
	}
	if path != "" {
		if err := manager.load(); err != nil {
			return nil, err
		}
	}
	return manager, nil
}

func (manager *Manager) RegisterProvider(provider core.AccountProvider) error {
	if provider == nil || strings.TrimSpace(string(provider.Type())) == "" {
		return errors.New("account provider and provider type are required")
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if _, exists := manager.providers[provider.Type()]; exists {
		return fmt.Errorf("account provider %q is already registered", provider.Type())
	}
	manager.providers[provider.Type()] = provider
	return nil
}

func (manager *Manager) Provider(accountType models.AccountType) (core.AccountProvider, error) {
	manager.mu.RLock()
	provider, exists := manager.providers[accountType]
	manager.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("%w: %q", core.ErrAccountOperationUnsupported, accountType)
	}
	return provider, nil
}

func (manager *Manager) AuthenticateAccount(ctx context.Context, request models.AccountAuthRequest) (models.Account, error) {
	provider, err := manager.Provider(request.Type)
	if err != nil {
		return models.Account{}, err
	}
	account, err := provider.Authenticate(ctx, request)
	if err != nil {
		return models.Account{}, err
	}
	if account.Type != request.Type || account.ID == "" {
		return models.Account{}, errors.New("account provider returned an invalid account")
	}
	if err := provider.Validate(ctx, account); err != nil {
		return models.Account{}, fmt.Errorf("account provider returned an invalid profile: %w", err)
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if _, exists := manager.accounts[account.ID]; exists {
		return models.Account{}, errors.New("account provider returned a duplicate account ID")
	}
	nextAccounts := cloneAccounts(manager.accounts)
	nextAccounts[account.ID] = cloneAccount(account)
	nextActiveID := manager.activeID
	if nextActiveID == "" {
		nextActiveID = account.ID
	}
	if err := manager.persistLocked(nextAccounts, nextActiveID); err != nil {
		return models.Account{}, err
	}
	manager.accounts, manager.activeID = nextAccounts, nextActiveID
	return cloneAccount(account), nil
}

func (manager *Manager) ListAccounts(ctx context.Context) ([]models.Account, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	accounts := make([]models.Account, 0, len(manager.accounts))
	for _, account := range manager.accounts {
		accounts = append(accounts, cloneAccount(account))
	}
	sort.Slice(accounts, func(i, j int) bool {
		if accounts[i].Name == accounts[j].Name {
			return accounts[i].ID < accounts[j].ID
		}
		return accounts[i].Name < accounts[j].Name
	})
	return accounts, nil
}

func (manager *Manager) GetAccount(ctx context.Context, id string) (models.Account, error) {
	if err := ctx.Err(); err != nil {
		return models.Account{}, err
	}
	manager.mu.RLock()
	account, exists := manager.accounts[id]
	manager.mu.RUnlock()
	if !exists {
		return models.Account{}, fmt.Errorf("account %q was not found", id)
	}
	return cloneAccount(account), nil
}

func (manager *Manager) ValidateAccount(ctx context.Context, id string) error {
	account, provider, err := manager.accountAndProvider(ctx, id)
	if err != nil {
		return err
	}
	return provider.Validate(ctx, account)
}

func (manager *Manager) RefreshAccount(ctx context.Context, id string) (models.Account, error) {
	account, provider, err := manager.accountAndProvider(ctx, id)
	if err != nil {
		return models.Account{}, err
	}
	refreshed, err := provider.Refresh(ctx, account)
	if err != nil {
		return models.Account{}, err
	}
	return manager.updateAccount(id, refreshed)
}

func (manager *Manager) GetAccountProfile(ctx context.Context, id string) (models.GameProfile, error) {
	account, provider, err := manager.accountAndProvider(ctx, id)
	if err != nil {
		return models.GameProfile{}, err
	}
	profile, err := provider.GetProfile(ctx, account)
	if err != nil {
		return models.GameProfile{}, err
	}
	return cloneProfile(profile), nil
}

func (manager *Manager) LogoutAccount(ctx context.Context, id string) error {
	account, provider, err := manager.accountAndProvider(ctx, id)
	if err != nil {
		return err
	}
	return provider.Logout(ctx, account)
}

func (manager *Manager) RemoveAccount(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if _, exists := manager.accounts[id]; !exists {
		return fmt.Errorf("account %q was not found", id)
	}
	nextAccounts := cloneAccounts(manager.accounts)
	delete(nextAccounts, id)
	nextActiveID := manager.activeID
	if nextActiveID == id {
		nextActiveID = ""
	}
	if err := manager.persistLocked(nextAccounts, nextActiveID); err != nil {
		return err
	}
	manager.accounts, manager.activeID = nextAccounts, nextActiveID
	return nil
}

func (manager *Manager) SetActiveAccount(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if _, exists := manager.accounts[id]; !exists {
		return fmt.Errorf("account %q was not found", id)
	}
	if err := manager.persistLocked(manager.accounts, id); err != nil {
		return err
	}
	manager.activeID = id
	return nil
}

func (manager *Manager) ActiveAccount(ctx context.Context) (models.Account, error) {
	if err := ctx.Err(); err != nil {
		return models.Account{}, err
	}
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	account, exists := manager.accounts[manager.activeID]
	if !exists {
		return models.Account{}, errors.New("no active account is selected")
	}
	return cloneAccount(account), nil
}

func (manager *Manager) accountAndProvider(ctx context.Context, id string) (models.Account, core.AccountProvider, error) {
	account, err := manager.GetAccount(ctx, id)
	if err != nil {
		return models.Account{}, nil, err
	}
	manager.mu.RLock()
	provider, exists := manager.providers[account.Type]
	manager.mu.RUnlock()
	if !exists {
		return models.Account{}, nil, fmt.Errorf("%w: %q", core.ErrAccountOperationUnsupported, account.Type)
	}
	return account, provider, nil
}

func (manager *Manager) updateAccount(id string, account models.Account) (models.Account, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if _, exists := manager.accounts[id]; !exists || account.ID != id {
		return models.Account{}, errors.New("account provider returned an invalid refreshed account")
	}
	account.UpdatedAt = time.Now().UTC()
	nextAccounts := cloneAccounts(manager.accounts)
	nextAccounts[id] = cloneAccount(account)
	if err := manager.persistLocked(nextAccounts, manager.activeID); err != nil {
		return models.Account{}, err
	}
	manager.accounts = nextAccounts
	return cloneAccount(account), nil
}

func (manager *Manager) load() error {
	file, err := os.Open(manager.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read account store: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("inspect account store: %w", err)
	}
	if info.Size() > maximumAccountStoreSize {
		return errors.New("account store exceeds 4 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(file, maximumAccountStoreSize+1))
	if err != nil {
		return fmt.Errorf("read account store: %w", err)
	}
	if len(data) > maximumAccountStoreSize {
		return errors.New("account store exceeds 4 MiB")
	}
	var store accountStore
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&store); err != nil {
		return fmt.Errorf("decode account store: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = errors.New("multiple JSON values are not allowed")
		}
		return fmt.Errorf("account store must contain one JSON object: %w", err)
	}
	if store.Version != accountStoreVersion {
		return fmt.Errorf("unsupported account store version %d", store.Version)
	}
	accounts := make(map[string]models.Account, len(store.Accounts))
	for _, account := range store.Accounts {
		if account.ID == "" || account.Type != models.AccountOffline {
			return fmt.Errorf("account store contains an unsupported or invalid account")
		}
		if err := NewOfflineProvider().Validate(context.Background(), account); err != nil {
			return fmt.Errorf("account store contains invalid account %q: %w", account.ID, err)
		}
		if _, exists := accounts[account.ID]; exists {
			return fmt.Errorf("account store contains duplicate account ID %q", account.ID)
		}
		accounts[account.ID] = cloneAccount(account)
	}
	if store.ActiveID != "" {
		if _, exists := accounts[store.ActiveID]; !exists {
			return errors.New("account store references a missing active account")
		}
	}
	manager.accounts, manager.activeID = accounts, store.ActiveID
	return nil
}

func (manager *Manager) persistLocked(accounts map[string]models.Account, activeID string) error {
	if manager.path == "" {
		return nil
	}
	directory := filepath.Dir(manager.path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return fmt.Errorf("create account store directory: %w", err)
	}
	records := make([]models.Account, 0, len(accounts))
	for _, account := range accounts {
		records = append(records, cloneAccount(account))
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	data, err := json.MarshalIndent(accountStore{Version: accountStoreVersion, ActiveID: activeID, Accounts: records}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode account store: %w", err)
	}
	file, err := os.CreateTemp(directory, ".accounts-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary account store: %w", err)
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return fmt.Errorf("restrict account store permissions: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return fmt.Errorf("write account store: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync account store: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close account store: %w", err)
	}
	if err := os.Rename(tempPath, manager.path); err != nil {
		return fmt.Errorf("replace account store: %w", err)
	}
	return nil
}

func cloneAccounts(accounts map[string]models.Account) map[string]models.Account {
	result := make(map[string]models.Account, len(accounts))
	for id, account := range accounts {
		result[id] = cloneAccount(account)
	}
	return result
}

func cloneAccount(account models.Account) models.Account {
	account.Profile = cloneProfile(account.Profile)
	return account
}

func cloneProfile(profile models.GameProfile) models.GameProfile {
	profile.Properties = append([]models.ProfileProperty(nil), profile.Properties...)
	return profile
}
