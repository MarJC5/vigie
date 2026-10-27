// Package store keeps Vigie's data in a local SQLite database (ADR-0006). It is the only package
// that talks SQL.
package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// DataDir returns the directory holding Vigie's data: the override when set, otherwise the user
// configuration directory followed by "Vigie" on macOS and "vigie" elsewhere.
// configDir is os.UserConfigDir in production.
func DataDir(goos string, configDir func() (string, error), override string) (string, error) {
	if override != "" {
		return override, nil
	}
	base, err := configDir()
	if err != nil {
		return "", fmt.Errorf("user configuration directory: %w", err)
	}
	name := "vigie"
	if goos == "darwin" {
		name = "Vigie"
	}
	return filepath.Join(base, name), nil
}

// EnsureDataDir creates the data directory, readable by its owner only: it holds hostnames and
// SSH user names.
func EnsureDataDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.Chmod(dir, 0o700)
}
