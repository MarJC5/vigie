package app

import (
	"strings"

	"github.com/MarJC5/vigie/internal/model"
)

// ChangeView is one thing that changed on a site since its previous complete scan (ADR-0027).
type ChangeView struct {
	Kind    string       `json:"kind"`    // model.ChangeKind
	Subject string       `json:"subject"` // core, plugin:<slug>, theme:<slug>, user:<login>, or a finding's
	Name    string       `json:"name"`    // the plugin's or theme's title, else its slug; an administrator's login; "" for WordPress
	From    string       `json:"from"`    // a version, or the theme active before
	To      string       `json:"to"`      // a version, or the theme active now
	ByVigie bool         `json:"byVigie"` // a version changed by an update or rollback of Vigie
	Super   bool         `json:"super"`   // an administrator added is a super administrator
	Finding *FindingView `json:"finding"` // the files or the new finding; nil otherwise
}

// SiteChangesView is what changed on one site with a scan of a run.
type SiteChangesView struct {
	SiteID  string       `json:"siteId"`
	Name    string       `json:"name"`
	Changes []ChangeView `json:"changes"`
}

// changeViews are the changes a scan made, each finding named as the change names its plugin or theme.
func changeViews(changes []model.Change) []ChangeView {
	views := []ChangeView{}
	for _, c := range changes {
		v := ChangeView{Kind: string(c.Kind), Subject: c.Subject, Name: c.Title, From: c.From, To: c.To, ByVigie: c.ByVigie, Super: c.Super}
		switch {
		case strings.HasPrefix(c.Subject, "user:"):
			v.Name = strings.TrimPrefix(c.Subject, "user:")
		case v.Name == "" && c.Subject != "core" && c.Finding == nil:
			_, v.Name, _ = strings.Cut(c.Subject, ":")
		}
		if c.Finding != nil {
			// The change names its plugin or theme as the scan did (model.Changes).
			f := findingView(*c.Finding, c.Title)
			v.Finding = &f
		}
		views = append(views, v)
	}
	return views
}
