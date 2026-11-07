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
	cleerrors "github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/errors"
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
	locksMu   sync.Mutex
	locks     map[string]*accountLock
}

func NewManager(path string, providers ...core.AccountProvider) (*Manager, error) {
	manager := &Manager{
		path: path, providers: make(map[models.AccountType]core.AccountProvider),
		accounts: make(map[string]models.Account), locks: make(map[string]*accountLock),
	}
	if err := manager.RegisterProvider(NewOfflineProvider()); err != nil {
		return nil, err
	}
	hasYggdrasilProvider := false
	hasMicrosoftProvider := false
	for _, provider := range providers {
		if err := manager.RegisterProvider(provider); err != nil {
			return nil, err
		}
		if provider.Type() == models.AccountYggdrasil {
			hasYggdrasilProvider = true
		}
		if provider.Type() == models.AccountMicrosoft {
			hasMicrosoftProvider = true
		}
	}
	if !hasYggdrasilProvider {
		yggdrasilProvider, err := NewYggdrasilProvider(YggdrasilOptions{Credentials: NewOSKeyring()})
		if err != nil {
			return nil, err
		}
		if err := manager.RegisterProvider(yggdrasilProvider); err != nil {
			return nil, err
		}
	}
	if !hasMicrosoftProvider {
		microsoftProvider, err := NewMicrosoftProvider(MicrosoftOptions{Credentials: NewOSKeyring()})
		if err != nil {
			return nil, err
		}
		if err := manager.RegisterProvider(microsoftProvider); err != nil {
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

// NewManagerWithClientID builds a manager whose Microsoft provider is configured
// with the given OAuth public-client application ID. When clientID is empty, the
// provider falls back to the CLE_MICROSOFT_CLIENT_ID environment variable.
func NewManagerWithClientID(path string, clientID string) (*Manager, error) {
	var providers []core.AccountProvider
	if strings.TrimSpace(clientID) != "" {
		microsoftProvider, err := NewMicrosoftProvider(MicrosoftOptions{Credentials: NewOSKeyring(), ClientID: clientID})
		if err != nil {
			return nil, err
		}
		providers = append(providers, microsoftProvider)
	}
	return NewManager(path, providers...)
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
		invalidAccountErr := errors.New("account provider returned an invalid account")
		if err := deleteAccountCredentials(ctx, provider, account); err != nil {
			return models.Account{}, errors.Join(invalidAccountErr, fmt.Errorf("clean up rejected account credentials: %w", err))
		}
		return models.Account{}, invalidAccountErr
	}
	if err := provider.Validate(ctx, account); err != nil {
		validationErr := fmt.Errorf("account provider returned an invalid profile: %w", err)
		if cleanupErr := deleteAccountCredentials(ctx, provider, account); cleanupErr != nil {
			return models.Account{}, errors.Join(validationErr, fmt.Errorf("clean up rejected account credentials: %w", cleanupErr))
		}
		return models.Account{}, validationErr
	}
	return manager.commitNewAccount(ctx, provider, account)
}

// AddAccount persists an account that was produced by a provider's dedicated
// interactive flow (for example the Microsoft OAuth device code flow), where
// credentials have already been stored by the provider.
func (manager *Manager) AddAccount(ctx context.Context, account models.Account) (models.Account, error) {
	if account.ID == "" || strings.TrimSpace(string(account.Type)) == "" {
		return models.Account{}, errors.New("account ID and type are required")
	}
	provider, err := manager.Provider(account.Type)
	if err != nil {
		return models.Account{}, err
	}
	if err := provider.Validate(ctx, account); err != nil {
		return models.Account{}, fmt.Errorf("account provider returned an invalid profile: %w", err)
	}
	return manager.commitNewAccount(ctx, provider, account)
}

func (manager *Manager) commitNewAccount(ctx context.Context, provider core.AccountProvider, account models.Account) (models.Account, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if _, exists := manager.accounts[account.ID]; exists {
		if err := deleteAccountCredentials(ctx, provider, account); err != nil {
			return models.Account{}, fmt.Errorf("duplicate account ID; clean up credentials: %w", err)
		}
		return models.Account{}, errors.New("account provider returned a duplicate account ID")
	}
	nextAccounts := cloneAccounts(manager.accounts)
	nextAccounts[account.ID] = cloneAccount(account)
	nextActiveID := manager.activeID
	if nextActiveID == "" {
		nextActiveID = account.ID
	}
	if err := manager.persistLocked(nextAccounts, nextActiveID); err != nil {
		if cleanupErr := deleteAccountCredentials(ctx, provider, account); cleanupErr != nil {
			return models.Account{}, errors.Join(err, fmt.Errorf("clean up credentials after account persistence failed: %w", cleanupErr))
		}
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
	unlock := manager.lockAccount(id)
	defer unlock()
	account, provider, err := manager.accountAndProvider(ctx, id)
	if err != nil {
		return err
	}
	return provider.Validate(ctx, account)
}

func (manager *Manager) RefreshAccount(ctx context.Context, id string) (models.Account, error) {
	unlock := manager.lockAccount(id)
	defer unlock()
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
	unlock := manager.lockAccount(id)
	defer unlock()
	account, provider, err := manager.accountAndProvider(ctx, id)
	if err != nil {
		return err
	}
	return provider.Logout(ctx, account)
}

func (manager *Manager) RemoveAccount(ctx context.Context, id string) error {
	unlock := manager.lockAccount(id)
	defer unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	manager.mu.Lock()
	if _, exists := manager.accounts[id]; !exists {
		manager.mu.Unlock()
		return fmt.Errorf("account %q was not found", id)
	}
	account := manager.accounts[id]
	nextAccounts := cloneAccounts(manager.accounts)
	delete(nextAccounts, id)
	nextActiveID := manager.activeID
	if nextActiveID == id {
		nextActiveID = ""
	}
	provider := manager.providers[account.Type]
	if err := manager.persistLocked(nextAccounts, nextActiveID); err != nil {
		manager.mu.Unlock()
		return err
	}
	manager.accounts, manager.activeID = nextAccounts, nextActiveID
	manager.mu.Unlock()
	if err := deleteAccountCredentials(ctx, provider, cloneAccount(account)); err != nil {
		return fmt.Errorf("account was removed, but its credentials could not be deleted from the operating-system credential store")
	}
	return nil
}

func (manager *Manager) ResolveLaunchAccount(ctx context.Context, id string) (models.Account, error) {
	unlock := manager.lockAccount(id)
	defer unlock()
	account, err := manager.GetAccount(ctx, id)
	if err != nil {
		return models.Account{}, cleerrors.Wrap(cleerrors.CodeAccountNotFound, "stored launch account was not found", err)
	}
	account = normalizeLaunchAccount(account)
	if !requiresSessionResolution(account.Type) {
		return account, nil
	}
	resolved, err := resolveStoredAccountWithProvider(ctx, manager, account)
	if err != nil {
		return models.Account{}, err
	}
	if !sameProfile(account.Profile, resolved.Profile) {
		session := resolved.Session
		resolved.Session = models.AccountSession{}
		resolved.UpdatedAt = time.Now().UTC()
		resolved, err = manager.updateAccount(id, resolved)
		if err != nil {
			return models.Account{}, err
		}
		resolved.Session = session
	}
	return resolved, nil
}

func (manager *Manager) lockAccount(id string) func() {
	manager.locksMu.Lock()
	lock := manager.locks[id]
	if lock == nil {
		lock = &accountLock{}
		manager.locks[id] = lock
	}
	lock.refs++
	manager.locksMu.Unlock()
	lock.mu.Lock()
	return func() {
		lock.mu.Unlock()
		manager.locksMu.Lock()
		lock.refs--
		if lock.refs == 0 {
			delete(manager.locks, id)
		}
		manager.locksMu.Unlock()
	}
}

func sameProfile(left, right models.GameProfile) bool {
	if left.ID != right.ID || left.Name != right.Name || len(left.Properties) != len(right.Properties) {
		return false
	}
	for index := range left.Properties {
		if left.Properties[index] != right.Properties[index] {
			return false
		}
	}
	return true
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
		if account.ID == "" {
			return fmt.Errorf("account store contains an unsupported or invalid account")
		}
		switch account.Type {
		case models.AccountOffline:
			if err := NewOfflineProvider().Validate(context.Background(), account); err != nil {
				return fmt.Errorf("account store contains invalid account %q: %w", account.ID, err)
			}
		case models.AccountYggdrasil:
			if err := validateYggdrasilAccount(account); err != nil {
				return fmt.Errorf("account store contains invalid account %q: %w", account.ID, err)
			}
			if _, err := validateYggdrasilServer(account.AuthServer, false); err != nil {
				return fmt.Errorf("account store contains invalid Yggdrasil server for account %q", account.ID)
			}
		case models.AccountMicrosoft:
			if err := validateMicrosoftAccount(account); err != nil {
				return fmt.Errorf("account store contains invalid account %q: %w", account.ID, err)
			}
		default:
			return fmt.Errorf("account store contains an unsupported or invalid account")
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

func deleteAccountCredentials(ctx context.Context, provider core.AccountProvider, account models.Account) error {
	cleaner, ok := provider.(interface {
		DeleteAccountCredentials(context.Context, models.Account) error
	})
	if !ok {
		return nil
	}
	return cleaner.DeleteAccountCredentials(context.WithoutCancel(ctx), account)
}

func cloneProfile(profile models.GameProfile) models.GameProfile {
	profile.Properties = append([]models.ProfileProperty(nil), profile.Properties...)
	return profile
}
