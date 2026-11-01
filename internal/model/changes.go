package model

import (
	"slices"
	"strconv"
)

// ChangeKind is what changed on a site between two of its scans (ADR-0027).
type ChangeKind string

const (
	ChangeAdminAdded    ChangeKind = "admin-added"    // Subject user:<login>; Super
	ChangeAdminRemoved  ChangeKind = "admin-removed"  // Subject user:<login>
	ChangeInstalled     ChangeKind = "installed"      // a plugin or theme: Vigie never installs one
	ChangeRemoved       ChangeKind = "removed"        // a plugin or theme
	ChangeThemeSwitched ChangeKind = "theme-switched" // Subject the theme now active; From, To its titles
	ChangeVersion       ChangeKind = "version"        // WordPress, a plugin or a theme; From, To, ByVigie
	ChangeFiles         ChangeKind = "files"          // WordPress's or a plugin's files newly different; Finding
	ChangeFinding       ChangeKind = "finding"        // a finding urgent or needing attention, new; Finding
)

// Change is one thing that changed on a site between two of its complete scans.
type Change struct {
	Kind    ChangeKind
	Subject string // core, plugin:<slug>, theme:<slug>, user:<login>, or a finding's
	Title   string // the plugin's or theme's title, when known
	From    string // a version, or the theme active before
	To      string // a version, or the theme active now
	ByVigie bool   // a version changed by an update or rollback of Vigie between the two scans
	Super   bool   // an administrator added is a super administrator
	Finding *Finding
}

// Changes compares a complete scan of a site with the complete scan before it, Vigie's operations
// on the site telling which versions it changed; nil when either is not complete (a first scan
// has nothing to compare with). The administrators first, then the plugins and themes, the
// versions, the files and the new findings.
func Changes(before, after Scan, operations []Operation) []Change {
	if before.Outcome != ScanCompleted || after.Outcome != ScanCompleted {
		return nil
	}
	changes := []Change{}
	changes = append(changes, administrators(before, after)...)
	changes = append(changes, items(before, after)...)
	changes = append(changes, versions(before, after, operations)...)
	changes = append(changes, files(before, after)...)
	changes = append(changes, findings(before, after)...)
	return changes
}

func administrators(before, after Scan) []Change {
	logins := func(s Scan) map[string]Finding {
		m := map[string]Finding{}
		for _, f := range s.Findings {
			if f.Kind == KindAdministrator {
				m[f.Subject] = f
			}
		}
		return m
	}
	was, is := logins(before), logins(after)
	var changes []Change
	for _, subject := range sortedKeys(is) {
		if _, ok := was[subject]; !ok {
			changes = append(changes, Change{Kind: ChangeAdminAdded, Subject: subject, Super: is[subject].Facts["super"] == "true"})
		}
	}
	for _, subject := range sortedKeys(was) {
		if _, ok := is[subject]; !ok {
			changes = append(changes, Change{Kind: ChangeAdminRemoved, Subject: subject})
		}
	}
	return changes
}

// extensions are the plugins and themes of a scan by subject.
func extensions(s Scan) map[string]ScanItem {
	m := map[string]ScanItem{}
	for _, item := range s.Items {
		if item.Kind == ItemPlugin || item.Kind == ItemTheme {
			m[string(item.Kind)+":"+item.Slug] = item
		}
	}
	return m
}

func items(before, after Scan) []Change {
	was, is := extensions(before), extensions(after)
	var changes []Change
	for _, subject := range sortedKeys(is) {
		if _, ok := was[subject]; !ok {
			changes = append(changes, Change{Kind: ChangeInstalled, Subject: subject, Title: is[subject].Title, To: is[subject].Version})
		}
	}
	for _, subject := range sortedKeys(was) {
		if _, ok := is[subject]; !ok {
			changes = append(changes, Change{Kind: ChangeRemoved, Subject: subject, Title: was[subject].Title, From: was[subject].Version})
		}
	}
	active := func(m map[string]ScanItem) (ScanItem, bool) {
		for _, subject := range sortedKeys(m) {
			if m[subject].Kind == ItemTheme && m[subject].Status == "active" {
				return m[subject], true
			}
		}
		return ScanItem{}, false
	}
	from, had := active(was)
	to, has := active(is)
	if had && has && from.Slug != to.Slug {
		changes = append(changes, Change{Kind: ChangeThemeSwitched, Subject: "theme:" + to.Slug, From: title(from), To: title(to)})
	}
	return changes
}

func versions(before, after Scan, operations []Operation) []Change {
	type versioned struct {
		subject, title, version string
		kind                    ItemKind
		slug                    string
	}
	list := func(s Scan) map[string]versioned {
		m := map[string]versioned{}
		for _, item := range s.Items {
			switch item.Kind {
			case ItemCore:
				m["core"] = versioned{subject: "core", version: item.Version, kind: ItemCore}
			case ItemPlugin, ItemTheme:
				subject := string(item.Kind) + ":" + item.Slug
				m[subject] = versioned{subject: subject, title: item.Title, version: item.Version, kind: item.Kind, slug: item.Slug}
			}
		}
		return m
	}
	was, is := list(before), list(after)
	// Vigie's own updates and rollbacks between the two scans.
	byVigie := func(kind ItemKind, slug string) bool {
		for _, op := range operations {
			if op.SiteID != after.SiteID || op.Started.Before(before.Finished) || op.Started.After(after.Finished) {
				continue
			}
			if slices.ContainsFunc(op.Steps, func(st OperationStep) bool { return st.ItemKind == kind && st.Slug == slug }) {
				return true
			}
		}
		return false
	}
	var changes []Change
	// By subject: WordPress ("core") first, then the plugins and the themes.
	for _, subject := range sortedKeys(is) {
		old, ok := was[subject]
		now := is[subject]
		if !ok || old.version == now.version || old.version == "" || now.version == "" {
			continue
		}
		changes = append(changes, Change{Kind: ChangeVersion, Subject: subject, Title: now.title, From: old.version, To: now.version,
			ByVigie: byVigie(now.kind, now.slug)})
	}
	return changes
}

func files(before, after Scan) []Change {
	was := map[string]Finding{}
	for _, f := range before.Findings {
		if f.Kind == KindCoreFile || f.Kind == KindPluginFile {
			was[f.Key()] = f
		}
	}
	var changes []Change
	for _, f := range after.Findings {
		if f.Kind != KindCoreFile && f.Kind != KindPluginFile {
			continue
		}
		old, ok := was[f.Key()]
		if ok && !more(old, f) {
			continue
		}
		changes = append(changes, Change{Kind: ChangeFiles, Subject: f.Subject, Finding: &f})
	}
	return changes
}

// more tells whether a files finding counts more files changed, added or missing than before.
func more(before, after Finding) bool {
	for _, fact := range []string{"changed", "added", "missing"} {
		was, _ := strconv.Atoi(before.Facts[fact])
		is, _ := strconv.Atoi(after.Facts[fact])
		if is > was {
			return true
		}
	}
	return false
}

func findings(before, after Scan) []Change {
	was := map[string]bool{}
	for _, f := range before.Findings {
		was[f.Key()] = true
	}
	var changes []Change
	for _, f := range after.Findings {
		switch {
		case f.Kind == KindAdministrator || f.Kind == KindCoreFile || f.Kind == KindPluginFile:
		case f.Ignore != nil || (f.Level != LevelUrgent && f.Level != LevelAttention):
		case was[f.Key()]:
		default:
			changes = append(changes, Change{Kind: ChangeFinding, Subject: f.Subject, Finding: &f})
		}
	}
	return changes
}

func title(item ScanItem) string {
	if item.Title != "" {
		return item.Title
	}
	return item.Slug
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
