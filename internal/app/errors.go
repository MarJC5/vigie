package app

import (
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/workspace"
)

// SiteErrorsView is a read of a site's errors, as the window shows it and the command gives it with
// --json: one shape for every door. Its texts from the log are already masked.
type SiteErrorsView struct {
	State  string           `json:"state"` // off, empty, left, read
	File   string           `json:"file"`  // relative to the site's folder, absolute outside it
	Size   int64            `json:"size"`
	Whole  bool             `json:"whole"` // the whole file was searched
	More   bool             `json:"more"`  // older errors than those read were left out
	Read   string           `json:"read"`  // RFC 3339
	Lines  int              `json:"lines"` // the errors and warnings read
	Since  string           `json:"since"` // the oldest of them, RFC 3339; "" when none
	Groups []ErrorGroupView `json:"groups"`
	Before []BeforeView     `json:"before"` // newest first
}

// ErrorGroupView is one group of errors: where it was raised is a lead, never a verdict.
type ErrorGroupView struct {
	Type    string `json:"type"` // fatal, parse, recoverable, warning
	Kind    string `json:"kind"` // plugin, theme, mu-plugin, wordpress, site, outside, unknown
	Slug    string `json:"slug"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Message string `json:"message"`
	Count   int    `json:"count"`
	First   string `json:"first"` // RFC 3339
	Last    string `json:"last"`
}

// BeforeView is one thing that changed on the site just before an error's first time.
type BeforeView struct {
	At        string           `json:"at"`        // RFC 3339
	Kind      string           `json:"kind"`      // update, rollback, or change
	Outcome   string           `json:"outcome"`   // of an update or a rollback
	Automatic bool             `json:"automatic"` // a rollback Vigie ran on its own
	Items     []BeforeItemView `json:"items"`     // what an update or a rollback changed
	Change    *ChangeView      `json:"change"`    // a change seen between two scans
}

// BeforeItemView is a plugin, theme or WordPress an update or a rollback changed.
type BeforeItemView struct {
	Kind    string `json:"kind"` // core, plugin, theme
	Slug    string `json:"slug"`
	From    string `json:"from"`
	To      string `json:"to"`
	Outcome string `json:"outcome"`
}

// SiteErrorsViewOf is the view of a read.
func SiteErrorsViewOf(e workspace.SiteErrors) SiteErrorsView {
	v := SiteErrorsView{
		State: string(e.State), File: e.File, Size: e.Size, Whole: e.Whole, More: e.More, Read: rfc3339(e.Read),
		Lines: e.Lines, Since: rfc3339(e.Since), Groups: []ErrorGroupView{}, Before: []BeforeView{},
	}
	for _, g := range e.Groups {
		v.Groups = append(v.Groups, ErrorGroupView{
			Type: string(g.Type), Kind: string(g.Kind), Slug: g.Slug, File: g.File, Line: g.Line,
			Message: g.Message, Count: g.Count, First: rfc3339(g.First), Last: rfc3339(g.Last),
		})
	}
	for _, b := range e.Before {
		item := BeforeView{At: rfc3339(b.At), Items: []BeforeItemView{}}
		switch {
		case b.Operation != nil:
			item.Kind, item.Outcome, item.Automatic = string(b.Operation.Kind), string(b.Operation.Outcome), b.Operation.Automatic
			for _, st := range b.Operation.Steps {
				if st.ItemKind != "" && (st.Step == model.StepUpdate || st.Step == model.StepRestoreFiles) {
					item.Items = append(item.Items, BeforeItemView{Kind: string(st.ItemKind), Slug: st.Slug,
						From: st.VersionBefore, To: st.VersionAfter, Outcome: string(st.Outcome)})
				}
			}
		case b.Change != nil:
			item.Kind = "change"
			c := changeViews([]model.Change{*b.Change})[0]
			item.Change = &c
		default:
			continue
		}
		v.Before = append(v.Before, item)
	}
	return v
}
