package app

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/MarJC5/vigie/internal/buildinfo"
	"github.com/MarJC5/vigie/internal/i18n"
)

// settingsStore is the part of the storage the preferences need.
type settingsStore interface {
	Setting(ctx context.Context, key string) (string, bool, error)
	SetSetting(ctx context.Context, key, value string) error
}

const (
	languageSetting   = "language"
	themeSetting      = "theme"
	backgroundSetting = "background"
	autoScanSetting   = "autoscan"
	notifySetting     = "notify"
)

// AutoScanHours are the intervals offered for automatic scans, in hours; 0 is off.
var AutoScanHours = []int{0, 1, 3, 6, 12, 24}

// The themes the window can take: the system's, or one the user chose.
const (
	ThemeSystem = "system"
	ThemeLight  = "light"
	ThemeDark   = "dark"
)

// Preferences serves the user's choices to the interface.
type Preferences struct {
	store  settingsStore
	system func() string
}

// NewPreferences returns the service; system gives the system language (i18n.SystemLanguage).
func NewPreferences(store settingsStore, system func() string) *Preferences {
	return &Preferences{store: store, system: system}
}

// Language returns the interface language: the stored choice, else the system language.
func (p *Preferences) Language(ctx context.Context) (string, error) {
	value, ok, err := p.store.Setting(ctx, languageSetting)
	if err != nil {
		return "", err
	}
	if lang := i18n.Supported(value); ok && lang != "" {
		return lang, nil
	}
	return p.system(), nil
}

// SetLanguage stores the interface language chosen by the user.
func (p *Preferences) SetLanguage(ctx context.Context, lang string) error {
	if i18n.Supported(lang) != lang {
		return fmt.Errorf("unsupported language %q", lang)
	}
	return p.store.SetSetting(ctx, languageSetting, lang)
}

// Theme returns the theme chosen for the window: ThemeSystem until the user chose another.
func (p *Preferences) Theme(ctx context.Context) (string, error) {
	value, ok, err := p.store.Setting(ctx, themeSetting)
	if err != nil {
		return "", err
	}
	if ok && (value == ThemeLight || value == ThemeDark) {
		return value, nil
	}
	return ThemeSystem, nil
}

// SetTheme stores the theme chosen for the window.
func (p *Preferences) SetTheme(ctx context.Context, theme string) error {
	if theme != ThemeSystem && theme != ThemeLight && theme != ThemeDark {
		return fmt.Errorf("unknown theme %q", theme)
	}
	return p.store.SetSetting(ctx, themeSetting, theme)
}

// BackgroundSettings are the user's choices for Vigie in the background, each off by default: the
// background mode (Vigie keeps running behind its menu bar icon once the window is closed),
// automatic scans of everything every AutoScan hours (0 for never), notifications.
type BackgroundSettings struct {
	On       bool `json:"on"`
	AutoScan int  `json:"autoScan"`
	Notify   bool `json:"notify"`
}

// Background returns the background settings.
func (p *Preferences) Background(ctx context.Context) (BackgroundSettings, error) {
	var b BackgroundSettings
	for key, set := range map[string]func(string){
		backgroundSetting: func(v string) { b.On = v == "on" },
		notifySetting:     func(v string) { b.Notify = v == "on" },
		autoScanSetting: func(v string) {
			if hours, err := strconv.Atoi(v); err == nil && slices.Contains(AutoScanHours, hours) {
				b.AutoScan = hours
			}
		},
	} {
		value, ok, err := p.store.Setting(ctx, key)
		if err != nil {
			return BackgroundSettings{}, err
		}
		if ok {
			set(value)
		}
	}
	return b, nil
}

// SetBackground stores the background settings; an interval not offered is refused.
func (p *Preferences) SetBackground(ctx context.Context, b BackgroundSettings) error {
	if !slices.Contains(AutoScanHours, b.AutoScan) {
		return fmt.Errorf("automatic scans every %d hours: not an interval offered", b.AutoScan)
	}
	onOff := map[bool]string{true: "on", false: "off"}
	for key, value := range map[string]string{
		backgroundSetting: onOff[b.On],
		notifySetting:     onOff[b.Notify],
		autoScanSetting:   strconv.Itoa(b.AutoScan),
	} {
		if err := p.store.SetSetting(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

// Version is Vigie's version, for the About tab of the settings.
func (p *Preferences) Version() string {
	return buildinfo.Version()
}
