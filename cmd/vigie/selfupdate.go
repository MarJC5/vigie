package main

import (
	"context"
	"encoding/base64"
	"os"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/endpoint"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/buildinfo"
)

// installKind tells who updates this Vigie (ADR-0038): a build of the sources nobody, Homebrew its
// cask, a Linux package its package manager, the app and the AppImage themselves.
func installKind(goos, version, appImage string, exists func(string) bool) app.InstallKind {
	switch {
	case version == "" || version == "dev":
		return app.InstallDev
	case goos == "darwin" && (exists("/opt/homebrew/Caskroom/vigie") || exists("/usr/local/Caskroom/vigie")):
		return app.InstallHomebrew
	case goos == "linux" && appImage == "":
		return app.InstallPackage
	}
	return app.InstallApp
}

// configureUpdates gives Wails' updater the signed manifest and the key in the app, without its own
// window: About shows the update. VIGIE_UPDATE_MANIFEST points elsewhere, to try a release from a
// local server; the signature is checked against the same key either way.
func configureUpdates(u *updater.Updater, version string, getenv func(string) string) error {
	key, err := base64.StdEncoding.DecodeString(buildinfo.UpdateKey)
	if err != nil {
		return err
	}
	provider, err := updateProvider(version, getenv)
	if err != nil {
		return err
	}
	return u.Init(updater.Config{CurrentVersion: strings.TrimPrefix(version, "v"), Providers: []updater.Provider{provider},
		PublicKey: key, Window: updater.WindowNone})
}

// updateProvider reads the signed manifest, at GitHub or where VIGIE_UPDATE_MANIFEST says, and says
// what it is, as every other request of Vigie.
func updateProvider(version string, getenv func(string) string) (*endpoint.Provider, error) {
	url := buildinfo.UpdateManifest
	if v := getenv("VIGIE_UPDATE_MANIFEST"); v != "" {
		url = v
	}
	agent := buildinfo.UserAgent(strings.TrimPrefix(version, "v"))
	return endpoint.New(endpoint.Config{URL: url, Headers: map[string]string{"User-Agent": agent}})
}

// appUpdater is the app's Wails updater, which exists once the app does.
type appUpdater struct{ app **application.App }

func (a appUpdater) Check(ctx context.Context) (*updater.Release, error) {
	return (*a.app).Updater.Check(ctx)
}
func (a appUpdater) DownloadAndInstall(ctx context.Context) error {
	return (*a.app).Updater.DownloadAndInstall(ctx)
}
func (a appUpdater) Restart(ctx context.Context) error { return (*a.app).Updater.Restart(ctx) }

// progressOf reads the payload of Wails' download progress event, given alone or in a list.
func progressOf(data any) (updater.Progress, bool) {
	switch d := data.(type) {
	case updater.Progress:
		return d, true
	case *updater.Progress:
		return *d, d != nil
	case []any:
		if len(d) > 0 {
			return progressOf(d[0])
		}
	}
	return updater.Progress{}, false
}

// exists tells whether a path exists.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
