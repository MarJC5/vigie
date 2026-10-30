// Command vigie monitors and updates WordPress sites over SSH and WP-CLI. Without a command it opens
// its window; with one, it runs it in the terminal.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/MarJC5/vigie/frontend"
	"github.com/MarJC5/vigie/internal/addons"
	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/buildinfo"
	"github.com/MarJC5/vigie/internal/cli"
	"github.com/MarJC5/vigie/internal/i18n"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/workspace"
	"github.com/MarJC5/vigie/locales"
)

// The events the window listens to, typed in its bindings.
func init() {
	application.RegisterEvent[app.ScansState](app.ScansEvent)
	application.RegisterEvent[app.UpdatesState](app.UpdatesEvent)
	application.RegisterEvent[app.DiscoveryProgress](app.DiscoveryEvent)
	application.RegisterEvent[app.MenuCommand](app.MenuEvent)
}

func main() {
	opts, err := cli.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "vigie:", err)
		os.Exit(3)
	}
	if !opts.OpensWindow() {
		os.Exit(runCommand(opts))
	}
	if err := openWindow(opts); err != nil {
		fmt.Fprintln(os.Stderr, "vigie:", err)
		os.Exit(1)
	}
}

// open opens the user's data: the default per-user directory, or --data-dir.
func open(ctx context.Context, opts cli.Options) (*workspace.Workspace, error) {
	dir, err := store.DataDir(runtime.GOOS, os.UserConfigDir, opts.DataDir)
	if err != nil {
		return nil, err
	}
	return workspace.Open(ctx, dir, buildinfo.Version())
}

// runCommand runs a command in the terminal, in the language of the terminal's locale.
func runCommand(opts cli.Options) int {
	texts, err := i18n.Load(locales.Files())
	if err != nil {
		fmt.Fprintln(os.Stderr, "vigie:", err)
		return 3
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return cli.Run(ctx, opts, cli.Env{
		In: os.Stdin, Out: os.Stdout, Err: os.Stderr, Lang: i18n.LanguageFromEnv(os.Getenv), Texts: texts,
		Open: func(ctx context.Context) (*workspace.Workspace, error) { return open(ctx, opts) },
	})
}

func openWindow(opts cli.Options) error {
	ws, err := open(context.Background(), opts)
	if err != nil {
		return err
	}
	defer ws.Close()
	db := ws.DB()

	// The scanner reports to the window, which exists once application.New has returned.
	var wails *application.App
	emit := func(name string, data any) {
		if wails != nil {
			wails.Event.Emit(name, data)
		}
	}
	scanner := app.NewScanner(ws, emit)
	defer scanner.CancelAll()

	preferences := app.NewPreferences(db, i18n.SystemLanguage)
	lang, err := preferences.Language(context.Background())
	if err != nil {
		return err
	}
	texts, err := i18n.Load(locales.Files())
	if err != nil {
		return err
	}
	// The native parts of the window take the theme chosen when the window is created: Wails sets
	// a window's appearance only then. The page itself follows a new choice at once.
	theme, err := preferences.Theme(context.Background())
	if err != nil {
		return err
	}

	files := picker{app: func() *application.App { return wails }}
	data := app.NewData(ws, files, time.Now)
	// Add-ons are looked for in the PATH and the usual install folders (an app opened from the Finder
	// does not get the PATH of the user's shell).
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	wails = application.New(application.Options{
		Name: "Vigie",
		Services: []application.Service{
			application.NewService(app.NewSiteManager(db)),
			application.NewService(preferences),
			application.NewService(app.NewHostWizard(ws, emit)),
			application.NewService(app.NewEditor(ws)),
			application.NewService(app.NewUpdates(ws, emit, scanner)),
			application.NewService(scanner),
			application.NewService(app.NewSecurity(ws, time.Now)),
			application.NewService(data),
			application.NewService(app.NewAddons(data, files, func() string { return addons.Folders(os.Getenv("PATH"), home) })),
		},
		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(frontend.Assets())},
		Mac:    application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	// Built by Vigie in the interface language, with its shortcuts; after application.New, which
	// the standard items (About) read.
	menu, err := menuBar(texts, lang, emit)
	if err != nil {
		return err
	}
	wails.Menu.Set(menu)
	wails.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Vigie",
		Width:     1280,
		Height:    800,
		MinWidth:  960,
		MinHeight: 600,
		URL:       "/",
		// The window buttons keep their standard place, on the navigation rail; the rail and the
		// header of the site list move the window (--wails-draggable in their styles).
		Mac: application.MacWindow{TitleBar: application.MacTitleBarHidden, Appearance: appearance(theme)},
	})
	return wails.Run()
}

// appearance is the macOS appearance of a theme; the system's by default.
func appearance(theme string) application.MacAppearanceType {
	switch theme {
	case app.ThemeLight:
		return application.NSAppearanceNameAqua
	case app.ThemeDark:
		return application.NSAppearanceNameDarkAqua
	}
	return application.DefaultAppearance
}
