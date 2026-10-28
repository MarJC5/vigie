// Command vigie monitors and updates WordPress sites over SSH and WP-CLI. Without a command it opens
// its window; with one, it runs it in the terminal.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/MarJC5/vigie/frontend"
	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/buildinfo"
	"github.com/MarJC5/vigie/internal/cli"
	"github.com/MarJC5/vigie/internal/store"
)

func main() {
	opts, err := cli.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "vigie:", err)
		os.Exit(2)
	}
	if !opts.OpensWindow() {
		os.Exit(cli.Run(opts, os.Stdout, os.Stderr))
	}
	if err := openWindow(opts); err != nil {
		fmt.Fprintln(os.Stderr, "vigie:", err)
		os.Exit(1)
	}
}

func openWindow(opts cli.Options) error {
	ctx := context.Background()
	dir, err := store.DataDir(runtime.GOOS, os.UserConfigDir, opts.DataDir)
	if err != nil {
		return err
	}
	if err := store.EnsureDataDir(dir); err != nil {
		return err
	}
	db, err := store.Open(ctx, filepath.Join(dir, "vigie.db"))
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Migrate(ctx, store.Migrations(), buildinfo.Version()); err != nil {
		return err
	}

	wails := application.New(application.Options{
		Name:     "Vigie",
		Services: []application.Service{application.NewService(app.NewSiteManager(db))},
		Assets:   application.AssetOptions{Handler: application.AssetFileServerFS(frontend.Assets())},
		Mac:      application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	wails.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Vigie",
		Width:     1280,
		Height:    800,
		MinWidth:  960,
		MinHeight: 600,
		URL:       "/",
		// The traffic lights sit over the navigation rail.
		Mac: application.MacWindow{InvisibleTitleBarHeight: 44, TitleBar: application.MacTitleBarHiddenInset},
	})
	return wails.Run()
}
