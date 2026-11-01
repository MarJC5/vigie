package store

import (
	"context"
	"database/sql"
	"fmt"
	"uuid"

	"github.com/MarJC5/vigie/internal/model"
)

// LoadTree reads the whole site manager.
func (db *DB) LoadTree(ctx context.Context) (*model.Tree, error) {
	tree := model.NewTree()
	folders, err := db.loadFolders(ctx)
	if err != nil {
		return nil, err
	}
	// Parents first: add the folders whose parent is already in the tree, until none is left.
	for len(folders) > 0 {
		var rest []model.Folder
		for _, f := range folders {
			if _, ok := tree.Folder(f.ParentID); f.ParentID == model.NoID || ok {
				if err := tree.AddFolder(f); err != nil {
					return nil, err
				}
				continue
			}
			rest = append(rest, f)
		}
		if len(rest) == len(folders) {
			return nil, fmt.Errorf("%d folders with a missing or circular parent", len(rest))
		}
		folders = rest
	}
	if err := db.loadHosts(ctx, tree); err != nil {
		return nil, err
	}
	return tree, db.loadSites(ctx, tree)
}

func (db *DB) loadFolders(ctx context.Context) ([]model.Folder, error) {
	rows, err := db.sql.QueryContext(ctx, "SELECT id, parent_id, name, color, notes, position FROM folders ORDER BY position, name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var folders []model.Folder
	for rows.Next() {
		var f model.Folder
		var id string
		var parent sql.NullString
		if err := rows.Scan(&id, &parent, &f.Name, &f.Color, &f.Notes, &f.Position); err != nil {
			return nil, err
		}
		if f.ID, err = uuid.Parse(id); err != nil {
			return nil, err
		}
		if f.ParentID, err = parseParent(parent); err != nil {
			return nil, err
		}
		folders = append(folders, f)
	}
	return folders, rows.Err()
}

func (db *DB) loadHosts(ctx context.Context, tree *model.Tree) error {
	rows, err := db.sql.QueryContext(ctx, `SELECT id, folder_id, name, alias, hostname, port, user, key_path, wp_cli,
		max_concurrent, host_key_fingerprint, color, notes, position FROM hosts ORDER BY position, name`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var h model.Host
		var id string
		var folder sql.NullString
		if err := rows.Scan(&id, &folder, &h.Name, &h.Alias, &h.Hostname, &h.Port, &h.User, &h.KeyPath, &h.WPCLI,
			&h.MaxConcurrent, &h.HostKeyFingerprint, &h.Color, &h.Notes, &h.Position); err != nil {
			return err
		}
		if h.ID, err = uuid.Parse(id); err != nil {
			return err
		}
		if h.FolderID, err = parseParent(folder); err != nil {
			return err
		}
		if err := tree.AddHost(h); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (db *DB) loadSites(ctx context.Context, tree *model.Tree) error {
	tags, err := db.loadTags(ctx)
	if err != nil {
		return err
	}
	subsites, err := db.loadSubsites(ctx)
	if err != nil {
		return err
	}
	rows, err := db.sql.QueryContext(ctx, `SELECT id, host_id, name, path, url, wp_cli, color, notes, position, multisite, admin_url
		FROM sites ORDER BY position, name`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var s model.Site
		var id, host string
		if err := rows.Scan(&id, &host, &s.Name, &s.Path, &s.URL, &s.WPCLI, &s.Color, &s.Notes, &s.Position, &s.Multisite, &s.AdminURL); err != nil {
			return err
		}
		if s.ID, err = uuid.Parse(id); err != nil {
			return err
		}
		if s.HostID, err = uuid.Parse(host); err != nil {
			return err
		}
		s.Tags, s.Subsites = tags[id], subsites[id]
		if err := tree.AddSite(s); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (db *DB) loadTags(ctx context.Context) (map[string][]string, error) {
	rows, err := db.sql.QueryContext(ctx, "SELECT site_id, tag FROM site_tags ORDER BY site_id, position")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := map[string][]string{}
	for rows.Next() {
		var site, tag string
		if err := rows.Scan(&site, &tag); err != nil {
			return nil, err
		}
		tags[site] = append(tags[site], tag)
	}
	return tags, rows.Err()
}

func (db *DB) loadSubsites(ctx context.Context) (map[string][]model.Subsite, error) {
	rows, err := db.sql.QueryContext(ctx, `SELECT site_id, blog_id, url, public, archived, spam, deleted, pinned
		FROM subsites ORDER BY site_id, blog_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	subsites := map[string][]model.Subsite{}
	for rows.Next() {
		var site string
		var s model.Subsite
		if err := rows.Scan(&site, &s.BlogID, &s.URL, &s.Public, &s.Archived, &s.Spam, &s.Deleted, &s.Pinned); err != nil {
			return nil, err
		}
		subsites[site] = append(subsites[site], s)
	}
	return subsites, rows.Err()
}
