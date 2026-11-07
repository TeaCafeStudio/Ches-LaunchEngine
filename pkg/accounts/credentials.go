package accounts

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"
)

const credentialService = "ChesLaunchEngine"

type CredentialStore interface {
	Store(context.Context, string, []byte) error
	Retrieve(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
}

type OSKeyring struct{}

func NewOSKeyring() *OSKeyring {
	return &OSKeyring{}
}

func (*OSKeyring) Store(ctx context.Context, key string, value []byte) error {
	if err := validateCredentialOperation(ctx, key); err != nil {
		return err
	}
	if len(value) == 0 {
		return errors.New("credential value cannot be empty")
	}
	if err := keyring.Set(credentialService, key, string(value)); err != nil {
		return fmt.Errorf("store credential in operating system keyring")
	}
	return nil
}

func (*OSKeyring) Retrieve(ctx context.Context, key string) ([]byte, error) {
	if err := validateCredentialOperation(ctx, key); err != nil {
		return nil, err
	}
	value, err := keyring.Get(credentialService, key)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, fmt.Errorf("credential was not found")
		}
		return nil, fmt.Errorf("retrieve credential from operating system keyring")
	}
	return []byte(value), nil
}

func (*OSKeyring) Delete(ctx context.Context, key string) error {
	if err := validateCredentialOperation(ctx, key); err != nil {
		return err
	}
	if err := keyring.Delete(credentialService, key); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("delete credential from operating system keyring")
	}
	return nil
}

func validateCredentialOperation(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(key) == "" {
		return errors.New("credential key cannot be empty")
	}
	return nil
}
