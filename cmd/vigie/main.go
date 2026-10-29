// Command vigie monitors and updates WordPress sites over SSH and WP-CLI. Without a command it opens
// its window; with one, it runs it in the terminal.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/MarJC5/vigie/frontend"
	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/buildinfo"
	"github.com/MarJC5/vigie/internal/cli"
	"github.com/MarJC5/vigie/internal/i18n"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/workspace"
	"github.com/MarJC5/vigie/locales"
)

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
		Out: os.Stdout, Err: os.Stderr, Lang: i18n.LanguageFromEnv(os.Getenv), Texts: texts,
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

	wails := application.New(application.Options{
		Name: "Vigie",
		Services: []application.Service{
			application.NewService(app.NewSiteManager(db)),
			application.NewService(app.NewPreferences(db, i18n.SystemLanguage)),
		},
		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(frontend.Assets())},
		Mac:    application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	wails.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Vigie",
		Width:     1280,
		Height:    800,
		MinWidth:  960,
		MinHeight: 600,
		URL:       "/",
		// The window buttons keep their standard place, on the navigation rail; the rail and the
		// header of the site list move the window (--wails-draggable in their styles).
		Mac: application.MacWindow{TitleBar: application.MacTitleBarHidden},
	})
	return wails.Run()
}
