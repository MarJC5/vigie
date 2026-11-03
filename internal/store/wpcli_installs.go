package store

import (
	"context"
	"time"

	"uuid"

	"github.com/MarJC5/vigie/internal/model"
)

// SaveWPCLIInstall records an install of WP-CLI by Vigie, done or failed (ADR-0039).
func (db *DB) SaveWPCLIInstall(ctx context.Context, i model.WPCLIInstall) error {
	completed := 0
	if i.Completed {
		completed = 1
	}
	_, err := db.sql.ExecContext(ctx, `INSERT INTO wpcli_installs
		(id, account, host_name, started_at, finished_at, completed, version, command, failed_step, message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		i.ID.String(), i.Account, i.HostName, timeText(i.Started), timeText(i.Finished), completed, i.Version, i.Command, i.FailedStep, i.Message)
	return err
}

// WPCLIInstalls returns the installs of WP-CLI started since a time, oldest first.
func (db *DB) WPCLIInstalls(ctx context.Context, since time.Time) ([]model.WPCLIInstall, error) {
	rows, err := db.sql.QueryContext(ctx, `SELECT id, account, host_name, started_at, finished_at, completed, version, command, failed_step, message
		FROM wpcli_installs WHERE started_at >= ? ORDER BY started_at, id`, timeText(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var installs []model.WPCLIInstall
	for rows.Next() {
		var i model.WPCLIInstall
		var id, started, finished string
		var completed int
		if err := rows.Scan(&id, &i.Account, &i.HostName, &started, &finished, &completed, &i.Version, &i.Command, &i.FailedStep, &i.Message); err != nil {
			return nil, err
		}
		if i.ID, err = uuid.Parse(id); err != nil {
			return nil, err
		}
		if i.Started, err = time.Parse(time.RFC3339Nano, started); err != nil {
			return nil, err
		}
		if i.Finished, err = time.Parse(time.RFC3339Nano, finished); err != nil {
			return nil, err
		}
		i.Completed = completed == 1
		installs = append(installs, i)
	}
	return installs, rows.Err()
}
