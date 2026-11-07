package accounts

import (
	"fmt"
	"os"
	"path/filepath"
)

func DefaultStorePath() (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user configuration directory: %w", err)
	}
	return filepath.Join(directory, "ChesLaunchEngine", "accounts.json"), nil
}
