package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"uuid"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/MarJC5/vigie/internal/model"
)

var (
	// ErrCycle is returned when a folder would be moved into itself or one of its descendants.
	ErrCycle = errors.New("a folder cannot be moved into itself or its descendants")
	// ErrNotEmpty is returned when deleting a folder or host that still has children.
	ErrNotEmpty = errors.New("not empty")
)

// Repository stores the site manager. Its SQLite implementation is DB; another storage engine
// would implement the same contract.
type Repository interface {
	LoadTree(ctx context.Context) (*model.Tree, error)
	SaveFolder(ctx context.Context, f model.Folder) error
	SaveHost(ctx context.Context, h model.Host) error
	SaveSite(ctx context.Context, s model.Site) error
	DeleteFolder(ctx context.Context, id model.ID) error
	DeleteHost(ctx context.Context, id model.ID) error
	DeleteSite(ctx context.Context, id model.ID) error
}

var _ Repository = (*DB)(nil)

// parentArg stores a missing parent as NULL.
func parentArg(id model.ID) any {
	if id == model.NoID {
		return nil
	}
	return id.String()
}

func parseParent(s sql.NullString) (model.ID, error) {
	if !s.Valid {
		return model.NoID, nil
	}
	return uuid.Parse(s.String)
}

func duplicate(err error, name string) error {
	var e *sqlite.Error
	if errors.As(err, &e) && e.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
		return fmt.Errorf("%q: %w", name, model.ErrDuplicateName)
	}
	return err
}

// withTx runs fn in a transaction, committed only when fn succeeds.
func (db *DB) withTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func exists(ctx context.Context, tx *sql.Tx, table string, id model.ID) (bool, error) {
	var n int
	err := tx.QueryRowContext(ctx, "SELECT count(*) FROM "+table+" WHERE id = ?", id.String()).Scan(&n)
	return n > 0, err
}

// nameTaken compares names in Go (Unicode, without case), the same rule as the model.
func nameTaken(ctx context.Context, tx *sql.Tx, query string, parent any, self model.ID, name string) (bool, error) {
	rows, err := tx.QueryContext(ctx, query, parent, self.String())
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var other string
		if err := rows.Scan(&other); err != nil {
			return false, err
		}
		if model.NamesEqual(other, name) {
			return true, nil
		}
	}
	return false, rows.Err()
}

// Folders and hosts sharing a parent folder share one namespace.
const folderNamespace = `SELECT name FROM folders WHERE parent_id IS ?1 AND id <> ?2
	UNION ALL SELECT name FROM hosts WHERE folder_id IS ?1 AND id <> ?2`

// SaveFolder creates or updates a folder.
func (db *DB) SaveFolder(ctx context.Context, f model.Folder) error {
	if err := f.Validate(); err != nil {
		return err
	}
	return db.withTx(ctx, func(tx *sql.Tx) error { return saveFolder(ctx, tx, f) })
}

func saveFolder(ctx context.Context, tx *sql.Tx, f model.Folder) error {
	if f.ParentID != model.NoID {
		if err := checkNoCycle(ctx, tx, f.ID, f.ParentID); err != nil {
			return err
		}
	}
	taken, err := nameTaken(ctx, tx, folderNamespace, parentArg(f.ParentID), f.ID, f.Name)
	if err != nil {
		return err
	}
	if taken {
		return fmt.Errorf("%q: %w", f.Name, model.ErrDuplicateName)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO folders (id, parent_id, name, color, notes, position)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET parent_id = excluded.parent_id, name = excluded.name,
			color = excluded.color, notes = excluded.notes, position = excluded.position`,
		f.ID.String(), parentArg(f.ParentID), f.Name, int(f.Color), f.Notes, f.Position)
	return duplicate(err, f.Name)
}

// checkNoCycle walks up from the new parent: it must exist and must not be the folder itself.
func checkNoCycle(ctx context.Context, tx *sql.Tx, self, parent model.ID) error {
	for current := parent; current != model.NoID; {
		if current == self {
			return ErrCycle
		}
		var up sql.NullString
		err := tx.QueryRowContext(ctx, "SELECT parent_id FROM folders WHERE id = ?", current.String()).Scan(&up)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("parent folder %s: %w", current, model.ErrNotFound)
		}
		if err != nil {
			return err
		}
		if current, err = parseParent(up); err != nil {
			return err
		}
	}
	return nil
}

// SaveHost creates or updates a host.
func (db *DB) SaveHost(ctx context.Context, h model.Host) error {
	return db.withTx(ctx, func(tx *sql.Tx) error { return saveHost(ctx, tx, h) })
}

// AddHostWithSites stores a new host and its sites in one transaction: if one is refused,
// nothing is stored.
func (db *DB) AddHostWithSites(ctx context.Context, h model.Host, sites []model.Site) error {
	return db.withTx(ctx, func(tx *sql.Tx) error {
		if err := saveHost(ctx, tx, h); err != nil {
			return err
		}
		for _, s := range sites {
			if err := saveSite(ctx, tx, s); err != nil {
				return err
			}
		}
		return nil
	})
}

// AddSites stores new sites in one transaction: if one is refused, none is stored.
func (db *DB) AddSites(ctx context.Context, sites []model.Site) error {
	return db.withTx(ctx, func(tx *sql.Tx) error {
		for _, s := range sites {
			if err := saveSite(ctx, tx, s); err != nil {
				return err
			}
		}
		return nil
	})
}

func saveHost(ctx context.Context, tx *sql.Tx, h model.Host) error {
	if err := h.Validate(); err != nil {
		return err
	}
	if h.FolderID != model.NoID {
		ok, err := exists(ctx, tx, "folders", h.FolderID)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("folder %s: %w", h.FolderID, model.ErrNotFound)
		}
	}
	taken, err := nameTaken(ctx, tx, folderNamespace, parentArg(h.FolderID), h.ID, h.Name)
	if err != nil {
		return err
	}
	if taken {
		return fmt.Errorf("%q: %w", h.Name, model.ErrDuplicateName)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO hosts (id, folder_id, name, alias, hostname, port, user, key_path,
			wp_cli, max_concurrent, host_key_fingerprint, color, notes, position)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET folder_id = excluded.folder_id, name = excluded.name,
			alias = excluded.alias, hostname = excluded.hostname, port = excluded.port, user = excluded.user,
			key_path = excluded.key_path, wp_cli = excluded.wp_cli, max_concurrent = excluded.max_concurrent,
			host_key_fingerprint = excluded.host_key_fingerprint, color = excluded.color,
			notes = excluded.notes, position = excluded.position`,
		h.ID.String(), parentArg(h.FolderID), h.Name, h.Alias, h.Hostname, h.Port, h.User, h.KeyPath,
		h.WPCLI, h.MaxConcurrent, h.HostKeyFingerprint, int(h.Color), h.Notes, h.Position)
	return duplicate(err, h.Name)
}

// SaveSite creates or updates a site with its tags and subsites.
func (db *DB) SaveSite(ctx context.Context, s model.Site) error {
	return db.withTx(ctx, func(tx *sql.Tx) error { return saveSite(ctx, tx, s) })
}

func saveSite(ctx context.Context, tx *sql.Tx, s model.Site) error {
	if err := s.Validate(); err != nil {
		return err
	}
	ok, err := exists(ctx, tx, "hosts", s.HostID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("host %s: %w", s.HostID, model.ErrNotFound)
	}
	var host string
	err = tx.QueryRowContext(ctx, "SELECT host_id FROM sites WHERE id = ?", s.ID.String()).Scan(&host)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return err
	case host != s.HostID.String():
		return fmt.Errorf("site %q: %w", s.Name, model.ErrSiteMoved)
	}
	taken, err := nameTaken(ctx, tx, "SELECT name FROM sites WHERE host_id = ?1 AND id <> ?2",
		s.HostID.String(), s.ID, s.Name)
	if err != nil {
		return err
	}
	if taken {
		return fmt.Errorf("%q: %w", s.Name, model.ErrDuplicateName)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO sites (id, host_id, name, path, url, wp_cli, color, notes, position, multisite, admin_url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET host_id = excluded.host_id, name = excluded.name, path = excluded.path,
			url = excluded.url, wp_cli = excluded.wp_cli, color = excluded.color, notes = excluded.notes,
			position = excluded.position, multisite = excluded.multisite, admin_url = excluded.admin_url`,
		s.ID.String(), s.HostID.String(), s.Name, s.Path, s.URL, s.WPCLI, int(s.Color), s.Notes, s.Position, s.Multisite, s.AdminURL)
	if err != nil {
		return duplicate(err, s.Name)
	}
	return saveSiteChildren(ctx, tx, s)
}

// SaveTags replaces the tags of several sites at once (a tag renamed or removed on all of them):
// every site is changed, or none.
func (db *DB) SaveTags(ctx context.Context, tags map[model.ID][]string) error {
	return db.withTx(ctx, func(tx *sql.Tx) error {
		for id, siteTags := range tags {
			ok, err := exists(ctx, tx, "sites", id)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("site %s: %w", id, model.ErrNotFound)
			}
			if err := saveTags(ctx, tx, id.String(), siteTags); err != nil {
				return err
			}
		}
		return nil
	})
}

// saveTags replaces the tags of a site, in order, without repeats.
func saveTags(ctx context.Context, tx *sql.Tx, id string, tags []string) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM site_tags WHERE site_id = ?", id); err != nil {
		return err
	}
	seen := map[string]bool{}
	for i, tag := range tags {
		if seen[tag] {
			continue
		}
		seen[tag] = true
		if _, err := tx.ExecContext(ctx, "INSERT INTO site_tags (site_id, position, tag) VALUES (?, ?, ?)", id, i, tag); err != nil {
			return err
		}
	}
	return nil
}

// saveSiteChildren replaces the tags (in order, without repeats) and the subsites of a site.
func saveSiteChildren(ctx context.Context, tx *sql.Tx, s model.Site) error {
	id := s.ID.String()
	if err := saveTags(ctx, tx, id, s.Tags); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM subsites WHERE site_id = ?", id); err != nil {
		return err
	}
	for _, sub := range s.Subsites {
		_, err := tx.ExecContext(ctx, `INSERT INTO subsites (site_id, blog_id, url, public, archived, spam, deleted, pinned)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, sub.BlogID, sub.URL, sub.Public, sub.Archived, sub.Spam, sub.Deleted, sub.Pinned)
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteFolder deletes an empty folder.
func (db *DB) DeleteFolder(ctx context.Context, id model.ID) error {
	return db.deleteNode(ctx, "folders", id,
		"SELECT (SELECT count(*) FROM folders WHERE parent_id = ?1) + (SELECT count(*) FROM hosts WHERE folder_id = ?1)")
}

// DeleteHost deletes a host with its sites, and their tags, subsites and scans.
func (db *DB) DeleteHost(ctx context.Context, id model.ID) error {
	return db.withTx(ctx, func(tx *sql.Tx) error {
		ok, err := exists(ctx, tx, "hosts", id)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("hosts %s: %w", id, model.ErrNotFound)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM sites WHERE host_id = ?", id.String()); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "DELETE FROM hosts WHERE id = ?", id.String())
		return err
	})
}

// DeleteSite deletes a site with its tags, subsites and scans.
func (db *DB) DeleteSite(ctx context.Context, id model.ID) error {
	return db.deleteNode(ctx, "sites", id, "")
}

func (db *DB) deleteNode(ctx context.Context, table string, id model.ID, childrenQuery string) error {
	return db.withTx(ctx, func(tx *sql.Tx) error {
		ok, err := exists(ctx, tx, table, id)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%s %s: %w", table, id, model.ErrNotFound)
		}
		if childrenQuery != "" {
			var children int
			if err := tx.QueryRowContext(ctx, childrenQuery, id.String()).Scan(&children); err != nil {
				return err
			}
			if children > 0 {
				return fmt.Errorf("%s %s: %w", table, id, ErrNotEmpty)
			}
		}
		_, err = tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE id = ?", id.String())
		return err
	})
}
