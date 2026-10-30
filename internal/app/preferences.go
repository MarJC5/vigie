package app

import (
	"context"
	"fmt"

	"github.com/MarJC5/vigie/internal/buildinfo"
	"github.com/MarJC5/vigie/internal/i18n"
)

// settingsStore is the part of the storage the preferences need.
type settingsStore interface {
	Setting(ctx context.Context, key string) (string, bool, error)
	SetSetting(ctx context.Context, key, value string) error
}

const languageSetting = "language"

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

// Version is Vigie's version, for the About tab of the settings.
func (p *Preferences) Version() string {
	return buildinfo.Version()
}
