package update_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/update"
	"github.com/MarJC5/vigie/internal/wpcli"
	"github.com/MarJC5/vigie/internal/wpcli/wpclitest"
)

func job(fixture string) update.Job {
	return update.Job{
		Site:   model.Site{ID: model.NewID(), Name: fixture + ".example.test", Path: "~/sites/" + fixture, URL: "http://" + fixture + ".example.test"},
		Target: sshx.Target{Alias: "bench"}, HostKey: "host-1", Max: 2, WPCLI: "wp",
	}
}

// bench answers the reads of the updates site and its previews, with a dump tool on the server.
func bench(t *testing.T) *sshxtest.Fake {
	t.Helper()
	fake := sshxtest.NewFake()
	wpclitest.Replay(t, fake, "updates", "~/sites/updates", "wp")
	wpclitest.ReplayUpdates(t, fake, "preview-plugins", "preview-themes")
	fake.Set(remote.DumpTool(), sshx.Result{Stdout: []byte("/usr/bin/mariadb-dump\n")})
	return fake
}

func updater(runner sshx.Runner) update.Updater {
	return update.Updater{Runner: runner, Limiter: sshx.NewLimiter()}
}

func TestPreviewOfTheUpdatesSite(t *testing.T) {
	got, err := updater(bench(t)).Preview(context.Background(), job("updates"))
	if err != nil {
		t.Fatal(err)
	}
	want := update.Preview{
		Core: &update.Item{Kind: model.ItemCore, From: "7.1.3", To: "7.1.4"},
		Plugins: []update.Item{
			{Kind: model.ItemPlugin, Slug: "akismet", From: "5.3", To: "5.7.2"},
			{Kind: model.ItemPlugin, Slug: "fixture-breaks", From: "1.0.0", To: "2.0.0"},
			{Kind: model.ItemPlugin, Slug: "fixture-missing", From: "1.0.0", To: "9.9.9"},
			{Kind: model.ItemPlugin, Slug: "fixture-pro", From: "1.0.0", To: "1.1.0"},
		},
		Themes: []update.Item{{Kind: model.ItemTheme, Slug: "twentytwentyfour", From: "1.0", To: "1.6"}},
		Languages: []update.Item{
			{Kind: model.ItemLanguage, Language: "fr_FR"},
			{Kind: model.ItemLanguage, Slug: "akismet", Language: "fr_FR"},
		},
		DumpTool: true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Preview() =\n%+v\nwant\n%+v", got, want)
	}
}

// Every update starts with a database dump: without a dump tool, the preview says so.
func TestPreviewWithoutADumpTool(t *testing.T) {
	fake := bench(t)
	fake.Set(remote.DumpTool(), sshx.Result{ExitCode: 1})
	got, err := updater(fake).Preview(context.Background(), job("updates"))
	if err != nil || got.DumpTool {
		t.Fatalf("Preview() = %+v, %v; want no dump tool", got.DumpTool, err)
	}
}

// Only WordPress behind (the image's 7.1.3): WP-CLI prints nothing for the previews of plugins
// and themes.
func TestPreviewWhenOnlyWordPressIsBehind(t *testing.T) {
	fake := sshxtest.NewFake()
	wpclitest.Replay(t, fake, "plain", "~/sites/plain", "wp")
	for _, of := range []wpcli.Extensions{wpcli.ExtensionPlugins, wpcli.ExtensionThemes} {
		line, err := wpcli.UpdatePreview(of).Remote("wp", "~/sites/plain")
		if err != nil {
			t.Fatal(err)
		}
		fake.Set(line, sshx.Result{})
	}
	fake.Set(remote.DumpTool(), sshx.Result{Stdout: []byte("/usr/bin/mysqldump\n")})
	got, err := updater(fake).Preview(context.Background(), job("plain"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Core == nil || *got.Core != (update.Item{Kind: model.ItemCore, From: "7.1.3", To: "7.1.4"}) ||
		len(got.Plugins)+len(got.Themes)+len(got.Languages) != 0 || !got.DumpTool {
		t.Errorf("Preview() = %+v, want WordPress only", got)
	}
	if got.Pending() != 1 {
		t.Errorf("Pending() = %d, want 1", got.Pending())
	}
}

func TestPreviewWhenWordPressCannotLoad(t *testing.T) {
	fake := sshxtest.NewFake()
	wpclitest.Replay(t, fake, "broken", "~/sites/broken", "wp")
	if _, err := updater(fake).Preview(context.Background(), job("broken")); !errors.Is(err, wpcli.ErrCannotLoad) {
		t.Fatalf("Preview() = %v, want ErrCannotLoad", err)
	}
}

// A bulk run updates everything the preview offers.
func TestAll(t *testing.T) {
	got, err := updater(bench(t)).Preview(context.Background(), job("updates"))
	if err != nil {
		t.Fatal(err)
	}
	sel := update.All(got)
	want := update.Selection{Core: true, Plugins: []string{"akismet", "fixture-breaks", "fixture-missing", "fixture-pro"},
		Themes: []string{"twentytwentyfour"}, Languages: true}
	if !reflect.DeepEqual(sel, want) {
		t.Errorf("All() = %+v", sel)
	}
	if empty := update.All(update.Preview{}); !reflect.DeepEqual(empty, update.Selection{}) {
		t.Errorf("All(nothing) = %+v", empty)
	}
}
