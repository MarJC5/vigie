package store

import (
	"context"
	"encoding/json"
	"time"

	"uuid"

	"github.com/MarJC5/vigie/internal/model"
)

// SaveNotice records a notification sent.
func (db *DB) SaveNotice(ctx context.Context, n model.Notice) error {
	names, err := json.Marshal(n.Names)
	if err != nil {
		return err
	}
	_, err = db.sql.ExecContext(ctx, "INSERT INTO notices (id, run_id, kind, names, sent_at) VALUES (?, ?, ?, ?, ?)",
		n.ID.String(), optionalID(n.RunID), n.Kind, string(names), timeText(n.Sent))
	return err
}

// Notices returns the notifications sent since a time, oldest first.
func (db *DB) Notices(ctx context.Context, since time.Time) ([]model.Notice, error) {
	rows, err := db.sql.QueryContext(ctx, "SELECT id, run_id, kind, names, sent_at FROM notices WHERE sent_at >= ? ORDER BY sent_at, id",
		timeText(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var notices []model.Notice
	for rows.Next() {
		var n model.Notice
		var id, run, names, sent string
		if err := rows.Scan(&id, &run, &n.Kind, &names, &sent); err != nil {
			return nil, err
		}
		if n.ID, err = uuid.Parse(id); err != nil {
			return nil, err
		}
		if n.RunID, err = parseOptionalID(run); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(names), &n.Names); err != nil {
			return nil, err
		}
		if n.Sent, err = time.Parse(time.RFC3339Nano, sent); err != nil {
			return nil, err
		}
		notices = append(notices, n)
	}
	return notices, rows.Err()
}
