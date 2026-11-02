package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/updater"

	"github.com/MarJC5/vigie/internal/app"
)

// Who updates this Vigie (ADR-0038).
func TestInstallKind(t *testing.T) {
	none := func(string) bool { return false }
	cask := func(p string) bool { return p == "/opt/homebrew/Caskroom/vigie" }
	cases := []struct {
		name, goos, version, appImage string
		exists                        func(string) bool
		want                          app.InstallKind
	}{
		{"a build of the sources", "darwin", "dev", "", none, app.InstallDev},
		{"the app from the DMG", "darwin", "0.6.0", "", none, app.InstallApp},
		{"the app from Homebrew", "darwin", "0.6.0", "", cask, app.InstallHomebrew},
		{"a Linux package", "linux", "0.6.0", "", none, app.InstallPackage},
		{"the AppImage", "linux", "0.6.0", "/home/m/Vigie.AppImage", none, app.InstallApp},
	}
	for _, c := range cases {
		if got := installKind(c.goos, c.version, c.appImage, c.exists); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
}

// The progress of a download, as Wails' event carries it.
func TestProgressOf(t *testing.T) {
	p := updater.Progress{Written: 11, Total: 18}
	for _, data := range []any{p, &p, []any{p}} {
		if got, ok := progressOf(data); !ok || got != p {
			t.Errorf("progressOf(%#v) = %+v, %v", data, got, ok)
		}
	}
	if _, ok := progressOf("nothing"); ok {
		t.Error("progressOf(a string) = ok")
	}
}

// The updater accepts the manifest's address and the key in the app.
func TestConfigureUpdates(t *testing.T) {
	u := updater.New(nil)
	if err := configureUpdates(u, "v0.6.0", func(string) string { return "" }); err != nil {
		t.Fatal(err)
	}
	if got := u.CurrentVersion(); got != "0.6.0" {
		t.Errorf("CurrentVersion() = %q", got)
	}
}

// The check says what it is, as every other request of Vigie, and sends what Settings lists: the
// version, the system and the architecture.
func TestTheCheckSaysWhatItIs(t *testing.T) {
	var agent, query string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent, query = r.UserAgent(), r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"schemaVersion":1,"version":"0.6.0","artifacts":[]}`))
	}))
	defer server.Close()
	provider, err := updateProvider("v0.6.0", func(string) string { return server.URL + "/update.json" })
	if err != nil {
		t.Fatal(err)
	}
	_, _ = provider.Check(context.Background(), updater.CheckRequest{CurrentVersion: "0.6.0", Platform: "darwin", Arch: "arm64"})
	if agent != "Vigie/0.6.0 (+https://github.com/MarJC5/vigie)" {
		t.Errorf("User-Agent = %q", agent)
	}
	if query != "arch=arm64&platform=darwin&version=0.6.0" {
		t.Errorf("sent %q, want the version, the system and the architecture only", query)
	}
}
