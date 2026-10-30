package main

import (
	"context"
	_ "embed"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/MarJC5/vigie/internal/app"
)

// trayIcon is the eye of Vigie's glyph in black, a template image macOS draws light or dark.
//
//go:embed tray.png
var trayIcon []byte

// keeper keeps Vigie running in the background when the user turned the mode on: closing the
// window hides it and Vigie leaves the Dock, its eye stays in the menu bar with what it watches;
// with the mode off, closing the window quits as before.
type keeper struct {
	wails   *application.App
	window  *application.WebviewWindow
	dock    appDock
	prefs   *app.Preferences
	watch   *app.Watch
	scanner *app.Scanner
	label   func(key string, vars map[string]any) string

	mu           sync.Mutex // the eye is made and removed while refresh writes its menu
	tray         *application.SystemTray
	next, urgent *application.MenuItem
}

// start puts the eye in the menu bar while the mode is on, and takes the window's closing.
func (k *keeper) start(ctx context.Context) {
	// With the mode on, the window hides instead of closing; Vigie leaves the Dock until it opens.
	k.window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if b, err := k.prefs.Background(ctx); err == nil && b.On {
			e.Cancel()
			k.window.Hide()
			k.dock.HideAppIcon()
		}
	})
	// With the mode off, closing the window quits, as before (a cancelled closing never gets here).
	k.window.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) { k.wails.Quit() })
	// Opened again from the Finder or Spotlight while hidden.
	k.wails.Event.OnApplicationEvent(events.Mac.ApplicationShouldHandleReopen, func(*application.ApplicationEvent) { k.open() })
	k.prefs.OnBackground = func(b app.BackgroundSettings) { k.apply(b) }
	k.wails.Event.On(app.ScansEvent, func(*application.CustomEvent) { k.refresh(ctx) })

	if b, err := k.prefs.Background(ctx); err == nil && b.On {
		k.showEye()
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			k.refresh(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// appDock takes Vigie out of the Dock and back (none in the server build).
type appDock interface {
	ShowAppIcon()
	HideAppIcon()
}

// apply follows a change of the settings: the eye shows while the mode is on; turned off while
// the window is hidden, the window opens.
func (k *keeper) apply(b app.BackgroundSettings) {
	if b.On {
		k.showEye()
	} else {
		k.removeEye()
		if !k.window.IsVisible() {
			k.open()
		}
	}
	k.refresh(context.Background())
}

// showEye puts the eye and its menu in the menu bar, once.
func (k *keeper) showEye() {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.tray != nil {
		return
	}
	menu := application.NewMenu()
	menu.Add(k.label("tray.open", nil)).OnClick(func(*application.Context) { k.open() })
	menu.Add(k.label("tray.scanAll", nil)).OnClick(func(*application.Context) { k.scanner.Scan("") })
	menu.AddSeparator()
	k.next = menu.Add("").SetEnabled(false)
	k.urgent = menu.Add("").SetEnabled(false)
	menu.AddSeparator()
	menu.Add(k.label("tray.quit", nil)).OnClick(func(*application.Context) { k.wails.Quit() })
	k.tray = k.wails.SystemTray.New().SetTemplateIcon(trayIcon).SetMenu(menu)
	k.tray.SetTooltip("Vigie")
}

// removeEye takes the eye out of the menu bar.
func (k *keeper) removeEye() {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.tray != nil {
		k.tray.Destroy()
		k.tray, k.next, k.urgent = nil, nil, nil
	}
}

// open shows the window and Vigie in the Dock.
func (k *keeper) open() {
	k.dock.ShowAppIcon()
	k.window.Show()
	k.window.Focus()
}

// refresh writes what the eye's menu says: the next automatic scan, the urgent sites.
func (k *keeper) refresh(ctx context.Context) {
	state, err := k.watch.State(ctx)
	if err != nil {
		return
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.tray == nil {
		return
	}
	if next, err := time.Parse(time.RFC3339, state.Next); err == nil {
		k.next.SetLabel(k.label("tray.next", map[string]any{"time": clockTime(next.Local(), time.Now())}))
	} else {
		k.next.SetLabel(k.label("tray.nextOff", nil))
	}
	if state.Urgent > 0 {
		k.urgent.SetLabel(k.label("tray.urgent", map[string]any{"count": state.Urgent}))
	} else {
		k.urgent.SetLabel(k.label("tray.noneUrgent", nil))
	}
}

// clockTime is the time of day of t, with its date when it is not today.
func clockTime(t, now time.Time) string {
	if y, m, d := t.Date(); y == now.Year() && m == now.Month() && d == now.Day() {
		return t.Format("15:04")
	}
	return t.Format("02.01. 15:04")
}
