package app

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/errorlog"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/workspace"
)

// The view of a read, as the window and the command give it: its fields by name, times in RFC 3339,
// an update told by the items it changed, a change as the journal's.
func TestSiteErrorsView(t *testing.T) {
	at := time.Date(2026, 11, 3, 2, 14, 0, 0, time.UTC)
	update := model.Operation{Kind: model.OperationUpdate, Outcome: model.OperationCompleted, Steps: []model.OperationStep{
		{Step: model.StepBackupDatabase},
		{Step: model.StepUpdate, ItemKind: model.ItemPlugin, Slug: "woocommerce", VersionBefore: "9.8.1", VersionAfter: "9.8.5", Outcome: model.StepDone},
		{Step: model.StepCheck, ItemKind: model.ItemPlugin, Slug: "woocommerce", Outcome: model.StepDone},
	}}
	theme := model.Change{Kind: model.ChangeThemeSwitched, Subject: "theme:astra-child", Title: "Astra Child", From: "Astra", To: "Astra Child"}
	v := SiteErrorsViewOf(workspace.SiteErrors{
		State: workspace.ErrorsRead, File: "wp-content/debug.log", Size: 38200000, Read: at.Add(time.Hour), Lines: 3, Since: at,
		Groups: []errorlog.Group{{Type: errorlog.Fatal, Kind: errorlog.Theme, Slug: "astra-child", File: "wp-content/themes/astra-child/functions.php",
			Line: 42, Message: "Uncaught Error: x()", Count: 2, First: at, Last: at.Add(time.Minute)}},
		Before: []workspace.Before{{At: at.Add(-time.Hour), Operation: &update}, {At: at.Add(-2 * time.Hour), Change: &theme}},
	})
	got, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"state":"read","file":"wp-content/debug.log","size":38200000,"whole":false,"more":false,"read":"2026-11-03T03:14:00Z","lines":3,"since":"2026-11-03T02:14:00Z",` +
		`"groups":[{"type":"fatal","kind":"theme","slug":"astra-child","file":"wp-content/themes/astra-child/functions.php","line":42,"message":"Uncaught Error: x()","count":2,"first":"2026-11-03T02:14:00Z","last":"2026-11-03T02:15:00Z"}],` +
		`"before":[{"at":"2026-11-03T01:14:00Z","kind":"update","outcome":"completed","automatic":false,"items":[{"kind":"plugin","slug":"woocommerce","from":"9.8.1","to":"9.8.5","outcome":"done"}],"change":null},` +
		`{"at":"2026-11-03T00:14:00Z","kind":"change","outcome":"","automatic":false,"items":[],"change":{"kind":"theme-switched","subject":"theme:astra-child","name":"Astra Child","from":"Astra","to":"Astra Child","byVigie":false,"super":false,"finding":null}}]}`
	if string(got) != want {
		t.Errorf("view:\n%s\nwant:\n%s", got, want)
	}

	// No log: empty lists, never null, and no time for what was not read.
	empty, _ := json.Marshal(SiteErrorsViewOf(workspace.SiteErrors{State: workspace.ErrorsOff, File: "wp-content/debug.log", Read: at}))
	if want := `{"state":"off","file":"wp-content/debug.log","size":0,"whole":false,"more":false,"read":"2026-11-03T02:14:00Z","lines":0,"since":"","groups":[],"before":[]}`; string(empty) != want {
		t.Errorf("no log:\n%s\nwant:\n%s", empty, want)
	}
}
