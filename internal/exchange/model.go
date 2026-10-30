package exchange

import (
	"os"
	"path/filepath"
	"time"

	"github.com/MarJC5/vigie/internal/model"
)

// FolderOf is a folder of the model; parent is "" for the top of the export.
func FolderOf(f model.Folder, parent string) Folder {
	return Folder{ID: f.ID.String(), Parent: parent, Name: f.Name, Color: f.Color.String(), Notes: f.Notes, Position: f.Position}
}

// HostOf is a host of the model with the keys trusted for it; folder is "" for the top of the
// export.
func HostOf(h model.Host, folder string, keys []HostKey) Host {
	if keys == nil {
		keys = []HostKey{}
	}
	return Host{
		ID: h.ID.String(), Folder: folder, Name: h.Name, Alias: h.Alias, Hostname: h.Hostname, Port: h.Port, User: h.User,
		KeyPath: h.KeyPath, WPCLI: h.WPCLI, MaxConcurrent: h.MaxConcurrent, TrustedKeys: keys,
		Color: h.Color.String(), Notes: h.Notes, Position: h.Position,
	}
}

// SiteOf is a site of the model.
func SiteOf(s model.Site) Site {
	site := Site{
		ID: s.ID.String(), Host: s.HostID.String(), Name: s.Name, Path: s.Path, URL: s.URL, WPCLI: s.WPCLI,
		Color: s.Color.String(), Notes: s.Notes, Tags: s.Tags, Position: s.Position, Multisite: s.Multisite,
		Subsites: []Subsite{},
	}
	if site.Tags == nil {
		site.Tags = []string{}
	}
	for _, sub := range s.Subsites {
		site.Subsites = append(site.Subsites, Subsite{BlogID: sub.BlogID, URL: sub.URL, Public: sub.Public,
			Archived: sub.Archived, Spam: sub.Spam, Deleted: sub.Deleted, Pinned: sub.Pinned})
	}
	return site
}

// IgnoreOf is an ignore of the model.
func IgnoreOf(i model.Ignore) Ignore {
	return Ignore{Site: i.SiteID.String(), Key: i.Key, UntilVersion: i.UntilVersion, Reason: i.Reason, Created: i.Created.UTC()}
}

// ScanOf is a scan of the model, with what it read and found.
func ScanOf(s model.Scan) Scan {
	scan := Scan{
		ID: s.ID.String(), Site: s.SiteID.String(), Started: s.Started.UTC(), Finished: s.Finished.UTC(),
		Outcome: string(s.Outcome), Failure: s.Failure, Error: s.Error,
		WordPress: s.WordPress, PHPCLI: s.PHPCLI, WPCLI: s.WPCLI, Home: s.Home,
		CoreChecked: s.CoreChecked, PluginsChecked: s.PluginsChecked, ThemesChecked: s.ThemesChecked, LanguagesChecked: s.LanguagesChecked,
		FilesChecked: optional(s.FilesChecked), Items: []Item{}, Findings: []Finding{},
	}
	for _, i := range s.Items {
		scan.Items = append(scan.Items, Item{Kind: string(i.Kind), Slug: i.Slug, Title: i.Title, Status: i.Status,
			Version: i.Version, Language: i.Language, Update: string(i.Update), UpdateVersion: i.UpdateVersion,
			AutoUpdate: i.AutoUpdate, OnWordPressOrg: i.OnWordPressOrg})
	}
	for _, f := range s.Findings {
		facts := f.Facts
		if facts == nil {
			facts = map[string]string{}
		}
		scan.Findings = append(scan.Findings, Finding{Kind: string(f.Kind), Subject: f.Subject, Identifier: f.Identifier,
			Version: f.Version, Level: string(f.Level), Facts: facts})
	}
	return scan
}

// OperationOf is an update or a rollback of the model, without the process that ran it, which
// belongs to this machine.
func OperationOf(op model.Operation) Operation {
	o := Operation{
		ID: op.ID.String(), Site: op.SiteID.String(), Kind: string(op.Kind), Started: op.Started.UTC(),
		Finished: optional(op.Finished), Outcome: string(op.Outcome), Summary: op.Summary, BackupDir: op.BackupDir, Steps: []Step{},
	}
	for _, st := range op.Steps {
		o.Steps = append(o.Steps, Step{Step: string(st.Step), ItemKind: string(st.ItemKind), Slug: st.Slug,
			VersionBefore: st.VersionBefore, VersionAfter: st.VersionAfter, Started: st.Started.UTC(),
			Finished: optional(st.Finished), Outcome: string(st.Outcome), Message: st.Message})
	}
	return o
}

func optional(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	u := t.UTC()
	return &u
}

// WriteFile writes an export readable by its owner only (0600), to a temporary file in the same
// folder renamed at the end: an existing file never keeps wider permissions, and an interrupted
// export never leaves half a file.
func WriteFile(path string, f File) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".vigie-export-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := Write(tmp, f); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
