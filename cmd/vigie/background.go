package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/model"
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
	login   *app.LoginItem
	atLogin bool // started by the system at login (--at-login); macOS says it once launched
	watch   *app.Watch
	scanner *app.Scanner
	notify  *notifications.NotificationService
	record  func(model.Notice) error // keeps a notification sent, for the Activity page
	label   func(key string, vars map[string]any) string

	notifyStart sync.Once
	notifyErr   error // why notifications cannot be sent here (no bundle, the server build)

	mu           sync.Mutex // the eye is made and removed while refresh writes its menu
	tray         *application.SystemTray
	next, urgent *application.MenuItem
}

// start puts the eye in the menu bar while the mode is on, and takes the window's closing. With the
// mode on the window is created hidden: it shows once Vigie has launched, unless at login.
func (k *keeper) start(ctx context.Context) {
	k.wails.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		if b, err := k.prefs.Background(ctx); err != nil || !b.On {
			return
		}
		if k.atLogin || launchedAtLogin() {
			k.dock.HideAppIcon()
			return
		}
		k.window.Show()
		k.window.Focus()
	})
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
	k.scanner.OnScanned = func(tree *model.Tree, before map[model.ID]model.Scan, after []model.Scan, changes map[model.ID][]model.Change) {
		if b, err := k.prefs.Background(ctx); err == nil && b.Notify {
			run := model.NoID
			if len(after) > 0 {
				run = after[0].RunID
			}
			k.alert(append(app.Alerts(tree, before, after), app.ChangeAlerts(tree, changes)...), run)
		}
	}

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

// loginItems are the system's login items through Wails; at login Vigie gets --at-login where the
// system passes arguments (Linux), and is told by macOS otherwise.
type loginItems struct{ app func() *application.App }

func (l loginItems) Enable() error {
	return l.app().Autostart.EnableWithOptions(application.AutostartOptions{Arguments: []string{"--at-login"}})
}
func (l loginItems) Disable() error           { return l.app().Autostart.Disable() }
func (l loginItems) IsEnabled() (bool, error) { return l.app().Autostart.IsEnabled() }

// appDock takes Vigie out of the Dock and back (none in the server build).
type appDock interface {
	ShowAppIcon()
	HideAppIcon()
}

// apply follows a change of the settings: the eye shows while the mode is on; turned off while
// the window is hidden, the window opens.
func (k *keeper) apply(b app.BackgroundSettings) {
	if b.Notify {
		// The system asks the user once; refused, notifications stay silent.
		go func() {
			if !k.notifications() {
				return
			}
			if _, err := k.notify.RequestNotificationAuthorization(); err != nil {
				k.wails.Logger.Error("notifications", "error", err)
			}
		}()
	}
	if b.On {
		k.showEye()
	} else {
		k.removeEye()
		// Started at login only for the background mode.
		if err := k.login.Set(false); err != nil && !errors.Is(err, application.ErrAutostartNotSupported) {
			k.wails.Logger.Error("login item", "error", err)
		}
		if !k.window.IsVisible() {
			k.open()
		}
	}
	k.refresh(context.Background())
}

// notifications starts the notification service once, at its first use rather than as a service
// of the window: it needs a bundled app, and the window needs none of its methods.
func (k *keeper) notifications() bool {
	k.notifyStart.Do(func() {
		k.notifyErr = k.notify.ServiceStartup(context.Background(), application.ServiceOptions{})
		if k.notifyErr != nil {
			k.wails.Logger.Error("notifications", "error", k.notifyErr)
		}
	})
	return k.notifyErr == nil
}

// alert sends a notification per kind of alert: the sites turned urgent, the hosts unreachable.
func (k *keeper) alert(alerts []app.Alert, run model.ID) {
	if len(alerts) == 0 || !k.notifications() {
		return
	}
	for _, kind := range []string{app.AlertUrgent, app.AlertUnreachable, app.AlertAdmin, app.AlertInstalled} {
		var names []string
		for _, a := range alerts {
			if a.Kind == kind {
				names = append(names, a.Name)
			}
		}
		if len(names) == 0 {
			continue
		}
		body := k.label("notify."+kind, map[string]any{"count": len(names), "name": names[0], "names": strings.Join(names, ", ")})
		err := k.notify.SendNotification(notifications.NotificationOptions{
			ID: fmt.Sprintf("vigie-%s-%d", kind, time.Now().UnixNano()), Title: "Vigie", Body: body,
		})
		if err != nil {
			k.wails.Logger.Error("notifications", "error", err)
			continue
		}
		if err := k.record(model.Notice{ID: model.NewID(), RunID: run, Kind: kind, Names: names, Sent: time.Now().UTC()}); err != nil {
			k.wails.Logger.Error("notifications", "error", err)
		}
	}
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
