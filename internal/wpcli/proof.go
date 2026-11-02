package wpcli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Updates is what a stored update check is about.
type Updates string

const (
	UpdatesOfCore    Updates = "update_core"
	UpdatesOfPlugins Updates = "update_plugins"
	UpdatesOfThemes  Updates = "update_themes"
)

// UpdateData reads what WordPress stored after its last update check. --network reads the site
// transient, where a multisite network keeps it too; a missing value prints nothing on stdout.
func UpdateData(of Updates) Command {
	switch of {
	case UpdatesOfCore, UpdatesOfPlugins, UpdatesOfThemes:
	default:
		return Command{err: fmt.Errorf("update data of %q: %w", of, ErrBadArgument)}
	}
	return Command{args: []string{"transient", "get", string(of), "--network", "--format=json"}, skipPlugins: true, skipThemes: true}
}

// CoreCheckProven tells whether WordPress's last version check reached wordpress.org for the
// installed version: a success stores at least one offer and the version it checked; a failure
// from scratch stores none. (last_checked proves nothing: WordPress sets it before the request.)
func CoreCheckProven(out []byte, installed string) bool {
	var data struct {
		Updates        []json.RawMessage `json:"updates"`
		VersionChecked string            `json:"version_checked"`
	}
	return json.Unmarshal(out, &data) == nil && len(data.Updates) > 0 && data.VersionChecked == installed
}

// ExtensionsCheckProven tells whether the plugin or theme check that WP-CLI just ran reached
// wordpress.org: WP-CLI deletes the stored data first, and only a successful answer writes
// "checked" (the versions it checked) again.
func ExtensionsCheckProven(out []byte) bool {
	var data struct {
		Checked json.RawMessage `json:"checked"`
	}
	return json.Unmarshal(out, &data) == nil && len(data.Checked) > 0 && string(data.Checked) != "null"
}

// ParseUpdateTranslations reads the translation updates in UpdateData's output: those WordPress
// stored with the check, as its Updates screen lists them (wp_get_translation_updates). WordPress
// itself is the item "". No stored data, which prints nothing, has none.
func ParseUpdateTranslations(out []byte) ([]LanguageUpdate, error) {
	list := []LanguageUpdate{}
	if len(bytes.TrimSpace(out)) == 0 {
		return list, nil
	}
	var data struct {
		Translations []struct {
			Type     string `json:"type"`
			Slug     string `json:"slug"`
			Language string `json:"language"`
		} `json:"translations"`
	}
	if err := json.Unmarshal(out, &data); err != nil {
		return nil, fmt.Errorf("update data: %w", ErrUnexpectedOutput)
	}
	for _, t := range data.Translations {
		if t.Language == "" {
			return nil, fmt.Errorf("update data: translation without language: %w", ErrUnexpectedOutput)
		}
		item := t.Slug
		if t.Type == "core" {
			item = ""
		}
		list = append(list, LanguageUpdate{Item: item, Language: t.Language})
	}
	return list, nil
}

// PluginsKnown reads, in the plugin UpdateData's output, the plugins wordpress.org answered for,
// with an update ("response") or without ("no_update"), by their WP-CLI name: the plugin's folder,
// or its file without ".php". A closed or unknown plugin is in neither.
func PluginsKnown(out []byte) map[string]bool {
	var data struct {
		Response json.RawMessage `json:"response"`
		NoUpdate json.RawMessage `json:"no_update"`
	}
	known := map[string]bool{}
	if json.Unmarshal(out, &data) != nil {
		return known
	}
	for _, raw := range []json.RawMessage{data.Response, data.NoUpdate} {
		// PHP writes an empty array as [], a filled one as an object.
		var files map[string]json.RawMessage
		if json.Unmarshal(raw, &files) != nil {
			continue
		}
		for file := range files {
			name, _, inFolder := strings.Cut(file, "/")
			if !inFolder {
				name = strings.TrimSuffix(file, ".php")
			}
			known[name] = true
		}
	}
	return known
}
