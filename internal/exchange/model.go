package exchange

import (
	"os"
	"path/filepath"
	"time"
	"uuid"

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

// The model of each part of an export, with the IDs of the file: the import sets the ones it
// changes. Read checked the IDs and colors; a File made otherwise gets zero for what is not one.

// Model is the folder in the model; its parent is left to the import.
func (f Folder) Model() model.Folder {
	color, _ := model.ParseColor(f.Color)
	return model.Folder{ID: parseID(f.ID), Name: f.Name, Color: color, Notes: f.Notes, Position: f.Position}
}

// Model is the host in the model, without a trusted key: the import sets it once confirmed.
func (h Host) Model() model.Host {
	color, _ := model.ParseColor(h.Color)
	return model.Host{
		ID: parseID(h.ID), Name: h.Name, Alias: h.Alias, Hostname: h.Hostname, Port: h.Port, User: h.User,
		KeyPath: h.KeyPath, WPCLI: h.WPCLI, MaxConcurrent: h.MaxConcurrent, Color: color, Notes: h.Notes, Position: h.Position,
	}
}

// Model is the site in the model; its host is left to the import.
func (s Site) Model() model.Site {
	color, _ := model.ParseColor(s.Color)
	site := model.Site{
		ID: parseID(s.ID), Name: s.Name, Path: s.Path, URL: s.URL, WPCLI: s.WPCLI, Color: color, Notes: s.Notes,
		Tags: s.Tags, Position: s.Position, Multisite: s.Multisite,
	}
	for _, sub := range s.Subsites {
		site.Subsites = append(site.Subsites, model.Subsite{BlogID: sub.BlogID, URL: sub.URL, Public: sub.Public,
			Archived: sub.Archived, Spam: sub.Spam, Deleted: sub.Deleted, Pinned: sub.Pinned})
	}
	return site
}

// Model is the ignore in the model, for the site given.
func (i Ignore) Model(site model.ID) model.Ignore {
	return model.Ignore{SiteID: site, Key: i.Key, UntilVersion: i.UntilVersion, Reason: i.Reason, Created: i.Created}
}

// Model is the scan in the model, for the site given.
func (s Scan) Model(site model.ID) model.Scan {
	scan := model.Scan{
		ID: parseID(s.ID), SiteID: site, Started: s.Started, Finished: s.Finished, Outcome: model.ScanOutcome(s.Outcome),
		Failure: s.Failure, Error: s.Error, WordPress: s.WordPress, PHPCLI: s.PHPCLI, WPCLI: s.WPCLI, Home: s.Home,
		CoreChecked: s.CoreChecked, PluginsChecked: s.PluginsChecked, ThemesChecked: s.ThemesChecked,
		LanguagesChecked: s.LanguagesChecked, FilesChecked: value(s.FilesChecked),
	}
	for _, i := range s.Items {
		scan.Items = append(scan.Items, model.ScanItem{Kind: model.ItemKind(i.Kind), Slug: i.Slug, Title: i.Title,
			Status: i.Status, Version: i.Version, Language: i.Language, Update: model.UpdateState(i.Update),
			UpdateVersion: i.UpdateVersion, AutoUpdate: i.AutoUpdate, OnWordPressOrg: i.OnWordPressOrg})
	}
	for _, f := range s.Findings {
		scan.Findings = append(scan.Findings, model.Finding{Kind: model.FindingKind(f.Kind), Subject: f.Subject,
			Identifier: f.Identifier, Version: f.Version, Level: model.FindingLevel(f.Level), Facts: f.Facts})
	}
	return scan
}

// Model is the operation in the model, for the site given. One still running where it was
// exported runs nowhere here: it is interrupted.
func (o Operation) Model(site model.ID) model.Operation {
	op := model.Operation{
		ID: parseID(o.ID), SiteID: site, Kind: model.OperationKind(o.Kind), Started: o.Started, Finished: value(o.Finished),
		Outcome: model.OperationOutcome(o.Outcome), Summary: o.Summary, BackupDir: o.BackupDir,
	}
	if op.Outcome == model.OperationRunning {
		op.Outcome = model.OperationInterrupted
	}
	for i, st := range o.Steps {
		op.Steps = append(op.Steps, model.OperationStep{Position: i, Step: model.StepKind(st.Step),
			ItemKind: model.ItemKind(st.ItemKind), Slug: st.Slug, VersionBefore: st.VersionBefore,
			VersionAfter: st.VersionAfter, Started: st.Started, Finished: value(st.Finished),
			Outcome: model.StepOutcome(st.Outcome), Message: st.Message})
	}
	return op
}

func parseID(s string) model.ID {
	id, err := uuid.Parse(s)
	if err != nil {
		return model.NoID
	}
	return id
}

func value(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
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
