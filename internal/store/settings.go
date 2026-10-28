package store

import (
	"context"
	"database/sql"
	"errors"
)

// Setting returns the value of an application setting, and whether it is set.
func (db *DB) Setting(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := db.sql.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return value, err == nil, err
}

// SetSetting stores the value of an application setting.
func (db *DB) SetSetting(ctx context.Context, key, value string) error {
	_, err := db.sql.ExecContext(ctx,
		"INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value", key, value)
	return err
}
