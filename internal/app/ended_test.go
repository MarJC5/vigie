package app_test

import (
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/i18n"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/locales"
)

// The end of a run notifies when the user started it, it lasted 30 s or more, and nobody stopped it
// (ADR-0035).
func TestRunEndNotifiesLongRunsOfTheUser(t *testing.T) {
	start := time.Date(2026, 11, 2, 14, 30, 0, 0, time.UTC)
	long := app.RunEnd{Kind: app.EndScan, By: model.RunByUser, Started: start, Finished: start.Add(30 * time.Second)}
	cases := []struct {
		name string
		end  func(e app.RunEnd) app.RunEnd
		want bool
	}{
		{"30 s by the user", func(e app.RunEnd) app.RunEnd { return e }, true},
		{"29 s", func(e app.RunEnd) app.RunEnd { e.Finished = start.Add(29 * time.Second); return e }, false},
		{"automatic", func(e app.RunEnd) app.RunEnd { e.By = model.RunBySchedule; return e }, false},
		{"command line", func(e app.RunEnd) app.RunEnd { e.By = model.RunByCLI; return e }, false},
		{"stopped", func(e app.RunEnd) app.RunEnd { e.Cancelled = true; return e }, false},
	}
	for _, c := range cases {
		if got := c.end(long).Notifies(); got != c.want {
			t.Errorf("%s: Notifies() = %v, want %v", c.name, got, c.want)
		}
	}
}

// Each kind of end says what ended in its title and how in its body, the run's alerts after it,
// in both languages.
func TestEndNoticeTexts(t *testing.T) {
	texts, err := i18n.Load(locales.Files())
	if err != nil {
		t.Fatal(err)
	}
	urgent := []app.Alert{{Kind: app.AlertUrgent, Name: "boulangerie-exemple.ch"}}
	cases := []struct {
		name        string
		lang        string
		end         app.RunEnd
		alerts      []app.Alert
		title, body string
	}{
		{
			"a scan of everything", "fr",
			app.RunEnd{Kind: app.EndScan, NodeIDs: []string{""}, Sites: 12, Urgent: 1}, urgent,
			"Relevé terminé", "Tous les sites : 12 sites relevés, 1 urgent. boulangerie-exemple.ch est devenu urgent.",
		},
		{
			"a scan of everything", "en",
			app.RunEnd{Kind: app.EndScan, NodeIDs: []string{""}, Sites: 12, Urgent: 1}, urgent,
			"Scan done", "All sites: 12 sites scanned, 1 urgent. boulangerie-exemple.ch turned urgent.",
		},
		{
			"a scan of a folder, a host unreachable", "fr",
			app.RunEnd{Kind: app.EndScan, NodeIDs: []string{"f"}, Name: "Clients", Sites: 5, Unreachable: 1}, nil,
			"Relevé terminé", "Clients : 5 sites relevés, 1 hébergement injoignable.",
		},
		{
			"a scan of a selection", "en",
			app.RunEnd{Kind: app.EndScan, NodeIDs: []string{"a", "b", "c"}, Sites: 1}, nil,
			"Scan done", "3 items: 1 site scanned.",
		},
		{
			"an update of a site", "fr",
			app.RunEnd{Kind: app.EndUpdate, NodeIDs: []string{"s"}, Name: "boulangerie-exemple.ch", Items: 4}, nil,
			"Mise à jour terminée", "boulangerie-exemple.ch : 4 éléments mis à jour.",
		},
		{
			"an update of a site stopped at a failure", "fr",
			app.RunEnd{Kind: app.EndUpdate, NodeIDs: []string{"s"}, Name: "boulangerie-exemple.ch", Items: 1, Failed: 1, FailedKind: model.ItemPlugin, FailedItem: "WooCommerce", NotStarted: 2}, nil,
			"Mise à jour arrêtée", "boulangerie-exemple.ch : WooCommerce en échec, 2 éléments non lancés.",
		},
		{
			"an update of a site stopped at its last item", "en",
			app.RunEnd{Kind: app.EndUpdate, NodeIDs: []string{"s"}, Name: "boulangerie-exemple.ch", Failed: 1, FailedKind: model.ItemCore}, nil,
			"Update stopped", "boulangerie-exemple.ch: WordPress failed.",
		},
		{
			"an update of a site stopped at its backup", "fr",
			app.RunEnd{Kind: app.EndUpdate, NodeIDs: []string{"s"}, Name: "boulangerie-exemple.ch", Failed: 1, NotStarted: 3}, nil,
			"Mise à jour arrêtée", "boulangerie-exemple.ch\u00a0: une étape en échec, 3 éléments non lancés.",
		},
		{
			"an update of several sites", "fr",
			app.RunEnd{Kind: app.EndUpdate, Several: true, NodeIDs: []string{"f"}, Name: "Clients", Updated: 5, Failed: 1}, nil,
			"Mise à jour terminée", "Clients : 5 sites mis à jour, 1 en échec.",
		},
		{
			"a copy of several sites", "fr",
			app.RunEnd{Kind: app.EndCopy, Several: true, NodeIDs: []string{"a", "b"}, Updated: 4, Failed: 1}, nil,
			"Copie terminée", "2 éléments\u00a0: 4 sites copiés, 1 en échec.",
		},
		{
			"a copy of a site", "fr",
			app.RunEnd{Kind: app.EndCopy, NodeIDs: []string{"s"}, Name: "boulangerie-exemple.ch", Archives: 9}, nil,
			"Copie terminée", "boulangerie-exemple.ch\u00a0: 9 archives copiées.",
		},
		{
			"a copy of a site cut off", "en",
			app.RunEnd{Kind: app.EndCopy, NodeIDs: []string{"s"}, Name: "boulangerie-exemple.ch", Archives: 1, Failed: 1}, nil,
			"Copy cut off", "boulangerie-exemple.ch: 1 archive copied.",
		},
		{
			"an update of several sites, none failed", "en",
			app.RunEnd{Kind: app.EndUpdate, Several: true, NodeIDs: []string{""}, Updated: 1}, nil,
			"Update done", "All sites: 1 site updated.",
		},
	}
	for _, c := range cases {
		label := func(key string, vars map[string]any) string { return texts.T(c.lang, key, vars) }
		got := app.EndNotice(c.end, c.alerts, label)
		if got.Title != c.title || got.Body != c.body {
			t.Errorf("%s (%s): got %q / %q, want %q / %q", c.name, c.lang, got.Title, got.Body, c.title, c.body)
		}
	}
	// The journal's line: the title, then how, without what the entry names already.
	label := func(key string, vars map[string]any) string { return texts.T("fr", key, vars) }
	got := app.EndNotice(app.RunEnd{Kind: app.EndScan, NodeIDs: []string{""}, Sites: 12, Urgent: 1}, urgent, label)
	if want := "Relevé terminé, 12 sites relevés, 1 urgent. boulangerie-exemple.ch est devenu urgent."; got.Line != want {
		t.Errorf("Line = %q, want %q", got.Line, want)
	}
}
