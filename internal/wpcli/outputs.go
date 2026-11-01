package wpcli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
)

// ErrCannotLoad is returned when WP-CLI could not load WordPress (database unreachable, fatal
// error in the configuration); the error carries WP-CLI's message.
var ErrCannotLoad = errors.New("WordPress cannot be loaded")

// ErrNotWordPress is the kind of ErrCannotLoad where the site's folder is gone or holds no
// WordPress any more: the site was most likely removed from the server.
var ErrNotWordPress = errors.New("no WordPress at this path")

// Check turns a failed WP-CLI command into ErrCannotLoad with its message, also ErrNotWordPress when
// WP-CLI finds no WordPress at the path; a success gives nil.
func Check(res sshx.Result) error {
	if res.ExitCode == 0 {
		return nil
	}
	msg := strings.TrimSpace(string(res.Stderr))
	if msg == "" {
		msg = "exit status " + strconv.Itoa(res.ExitCode)
	}
	// WP-CLI's words, the same for a missing folder and a folder without wp-load.php.
	if strings.Contains(msg, "This does not seem to be a WordPress installation") {
		return fmt.Errorf("%w: %w: %s", ErrCannotLoad, ErrNotWordPress, msg)
	}
	return fmt.Errorf("%w: %s", ErrCannotLoad, msg)
}

// IsInstalled reads `core is-installed`, which answers with its exit code only: 1 with nothing
// on stderr means "no" (not installed, or not a network with --network), 1 with an error means
// WordPress could not be loaded.
func IsInstalled(res sshx.Result) (bool, error) {
	switch {
	case res.ExitCode == 0:
		return true, nil
	case res.ExitCode == 1 && len(bytes.TrimSpace(res.Stderr)) == 0:
		return false, nil
	}
	return false, Check(res)
}

// CoreCheckUpdate lists the available WordPress releases; "[]" when none (or when the check could
// not reach wordpress.org, which the scan verifies separately).
func CoreCheckUpdate() Command {
	return Command{args: []string{"core", "check-update", "--format=json"}, skipPlugins: true, skipThemes: true}
}

// PluginList lists the installed plugins with their update state and whether wordpress.org knows
// them. Not wporg_last_updated: WP-CLI reads it from Trac, one request per plugin.
func PluginList() Command {
	return Command{args: []string{"plugin", "list", "--format=json",
		"--fields=name,title,status,version,update,update_version,auto_update,wporg_status"}, skipPlugins: true, skipThemes: true}
}

// ThemeList lists the installed themes with their update state.
func ThemeList() Command {
	return Command{args: []string{"theme", "list", "--format=json",
		"--fields=name,title,status,version,update,update_version,auto_update"}, skipPlugins: true, skipThemes: true}
}

// Languages is what a translation list is about.
type Languages string

const (
	LanguagesOfCore    Languages = "core"
	LanguagesOfPlugins Languages = "plugin"
	LanguagesOfThemes  Languages = "theme"
)

// LanguageUpdates lists the installed translations that have an update.
func LanguageUpdates(of Languages) Command {
	args := []string{"language", string(of), "list"}
	switch of {
	case LanguagesOfCore:
	case LanguagesOfPlugins, LanguagesOfThemes:
		args = append(args, "--all")
	default:
		return Command{err: fmt.Errorf("languages of %q: %w", of, ErrBadArgument)}
	}
	return Command{args: append(args, "--update=available", "--format=json"), skipPlugins: true, skipThemes: true}
}

// SiteList lists the sites of a multisite network.
func SiteList() Command {
	return Command{args: []string{"site", "list", "--format=json",
		"--fields=blog_id,url,registered,last_updated,public,archived,spam,deleted"}, skipPlugins: true, skipThemes: true}
}

var version = regexp.MustCompile(`^[0-9]+(\.[0-9]+)*(-[A-Za-z0-9.]+)?$`)

// ParseCoreVersion reads `core version`: one version number.
func ParseCoreVersion(out []byte) (string, error) {
	v := strings.TrimSpace(string(out))
	if !version.MatchString(v) {
		return "", fmt.Errorf("core version %q: %w", v, ErrUnexpectedOutput)
	}
	return v, nil
}

// ParseHome reads `option get home`: an http or https address.
func ParseHome(out []byte) (string, error) {
	return parseAddress("home", out)
}

// ParseSiteURL reads `option get siteurl`, the WordPress address where wp-admin lives: an http
// or https address, the site's own or a subfolder of it.
func ParseSiteURL(out []byte) (string, error) {
	return parseAddress("siteurl", out)
}

func parseAddress(option string, out []byte) (string, error) {
	address := strings.TrimSpace(string(out))
	u, err := url.Parse(address)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("%s %q: %w", option, address, ErrUnexpectedOutput)
	}
	return address, nil
}

// ParseActiveTheme reads `option get stylesheet`: the folder name of the active theme, on one
// line; empty when the option is.
func ParseActiveTheme(out []byte) (string, error) {
	name := strings.TrimSpace(string(out))
	if strings.ContainsAny(name, "\n/") {
		return "", fmt.Errorf("stylesheet %q: %w", name, ErrUnexpectedOutput)
	}
	return name, nil
}

// CoreUpdate is an available WordPress release.
type CoreUpdate struct {
	Version string
	Type    string // "major" or "minor"
}

// ParseCoreUpdates reads `core check-update --format=json`.
func ParseCoreUpdates(out []byte) ([]CoreUpdate, error) {
	var raw []struct {
		Version    string `json:"version"`
		UpdateType string `json:"update_type"`
	}
	if err := decode(out, &raw); err != nil {
		return nil, fmt.Errorf("core check-update: %w", err)
	}
	updates := []CoreUpdate{}
	for _, r := range raw {
		if !version.MatchString(r.Version) {
			return nil, fmt.Errorf("core check-update: version %q: %w", r.Version, ErrUnexpectedOutput)
		}
		updates = append(updates, CoreUpdate{Version: r.Version, Type: r.UpdateType})
	}
	return updates, nil
}

// UpdateState is the "update" field of `plugin list` and `theme list`.
type UpdateState string

const (
	UpdateNone        UpdateState = "none"
	UpdateAvailable   UpdateState = "available"
	UpdateUnavailable UpdateState = "unavailable" // a newer version needs another PHP or WordPress
	// UpdateNewerThanSource: the installed version is newer than wordpress.org's.
	UpdateNewerThanSource UpdateState = "version higher than expected"
	UpdateUnknown         UpdateState = "unknown" // a value this version of Vigie does not know
)

// Extension is an installed plugin or theme.
type Extension struct {
	Name           string // slug
	Title          string
	Status         string // plugins: active, active-network, inactive, must-use, dropin; themes: active, parent, inactive
	Version        string
	Update         UpdateState
	UpdateVersion  string
	AutoUpdate     bool
	OnWordPressOrg bool // plugins only: wordpress.org knows the plugin (wporg_status "active")
}

type rawExtension struct {
	Name          string `json:"name"`
	Title         loose  `json:"title"`
	Status        string `json:"status"`
	Version       loose  `json:"version"`
	Update        loose  `json:"update"`
	UpdateVersion loose  `json:"update_version"`
	AutoUpdate    loose  `json:"auto_update"`
	WPOrgStatus   loose  `json:"wporg_status"`
}

// loose reads a field WP-CLI writes as a string for ordinary plugins, and as false or null for
// must-use plugins and dropins, which have no update of their own (extension-command
// Plugin_Command::get_all_items). false and null read as "", true as "on".
type loose string

func (l *loose) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*l = loose(text) // null leaves text empty
		return nil
	}
	var flag bool
	if err := json.Unmarshal(data, &flag); err != nil {
		return err
	}
	if flag {
		*l = "on"
	} else {
		*l = ""
	}
	return nil
}

func parseExtensions(what string, out []byte) ([]Extension, error) {
	var raw []rawExtension
	if err := decode(out, &raw); err != nil {
		return nil, fmt.Errorf("%s list: %w", what, err)
	}
	list := []Extension{}
	for _, r := range raw {
		if r.Name == "" || r.Status == "" {
			return nil, fmt.Errorf("%s list: entry without name or status: %w", what, ErrUnexpectedOutput)
		}
		state := UpdateState(r.Update)
		switch state {
		case UpdateNone, UpdateAvailable, UpdateUnavailable, UpdateNewerThanSource:
		case "": // must-use plugins and dropins: WordPress does not update them
			state = UpdateNone
		default:
			state = UpdateUnknown
		}
		list = append(list, Extension{
			Name: r.Name, Title: string(r.Title), Status: r.Status, Version: string(r.Version),
			Update: state, UpdateVersion: string(r.UpdateVersion),
			AutoUpdate: r.AutoUpdate == "on", OnWordPressOrg: r.WPOrgStatus == "active",
		})
	}
	return list, nil
}

// ParsePlugins reads PluginList's output.
func ParsePlugins(out []byte) ([]Extension, error) { return parseExtensions("plugin", out) }

// ParseThemes reads ThemeList's output.
func ParseThemes(out []byte) ([]Extension, error) { return parseExtensions("theme", out) }

// LanguageUpdate is an installed translation with an update.
type LanguageUpdate struct {
	Item     string // plugin or theme slug; empty for WordPress itself
	Language string // for example "fr_FR"
}

// noneInstalled are what LanguageUpdates prints instead of a list when WordPress sees no theme or
// no plugin (language-command, Theme_Language_Command and Plugin_Language_Command).
var noneInstalled = []string{"Success: No themes installed.", "Success: No plugins installed."}

// ParseLanguageUpdates reads LanguageUpdates's output.
func ParseLanguageUpdates(out []byte) ([]LanguageUpdate, error) {
	if slices.Contains(noneInstalled, string(bytes.TrimSpace(out))) {
		return []LanguageUpdate{}, nil
	}
	var raw []struct {
		Plugin   string `json:"plugin"`
		Theme    string `json:"theme"`
		Language string `json:"language"`
	}
	if err := decode(out, &raw); err != nil {
		return nil, fmt.Errorf("language list: %w", err)
	}
	list := []LanguageUpdate{}
	for _, r := range raw {
		if r.Language == "" {
			return nil, fmt.Errorf("language list: entry without language: %w", ErrUnexpectedOutput)
		}
		list = append(list, LanguageUpdate{Item: r.Plugin + r.Theme, Language: r.Language})
	}
	return list, nil
}

// ParseSites reads SiteList's output; WP-CLI prints the numbers and flags as strings.
func ParseSites(out []byte) ([]model.Subsite, error) {
	var raw []struct {
		BlogID   string `json:"blog_id"`
		URL      string `json:"url"`
		Public   string `json:"public"`
		Archived string `json:"archived"`
		Spam     string `json:"spam"`
		Deleted  string `json:"deleted"`
	}
	if err := decode(out, &raw); err != nil {
		return nil, fmt.Errorf("site list: %w", err)
	}
	sites := []model.Subsite{}
	for _, r := range raw {
		id, err := strconv.Atoi(r.BlogID)
		if err != nil || id <= 0 || r.URL == "" {
			return nil, fmt.Errorf("site list: blog %q at %q: %w", r.BlogID, r.URL, ErrUnexpectedOutput)
		}
		sites = append(sites, model.Subsite{
			BlogID: id, URL: r.URL,
			Public: r.Public == "1", Archived: r.Archived == "1", Spam: r.Spam == "1", Deleted: r.Deleted == "1",
		})
	}
	return sites, nil
}

// decode reads a JSON array; anything else is ErrUnexpectedOutput.
func decode(out []byte, v any) error {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return ErrUnexpectedOutput
	}
	if err := json.Unmarshal(trimmed, v); err != nil {
		return fmt.Errorf("%w: %v", ErrUnexpectedOutput, err)
	}
	return nil
}
