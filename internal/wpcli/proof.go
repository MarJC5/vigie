package wpcli

import (
	"encoding/json"
	"fmt"
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
