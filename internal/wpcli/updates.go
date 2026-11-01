package wpcli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/MarJC5/vigie/internal/sshx"
)

var (
	// ErrFailed is returned when a command that changes a site failed; the error carries its message.
	ErrFailed = errors.New("WP-CLI command failed")
	// ErrNoDumpTool is returned when the server has neither mariadb-dump nor mysqldump.
	ErrNoDumpTool = errors.New("no database dump tool on the server")
)

// Extensions is what an update is about: plugins or themes.
type Extensions string

const (
	ExtensionPlugins Extensions = "plugin"
	ExtensionThemes  Extensions = "theme"
)

// A plugin or theme folder name, as WordPress and wordpress.org write them.
var slugName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// A constant of wp-config.php.
var constantName = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// A database dump in Vigie's backups folder, relative to the account's home directory, for
// example .vigie/backups/<site>/<time>/database.sql; each name starts with a letter or digit, so
// ".." cannot climb out.
var backupFile = regexp.MustCompile(`^\.vigie/backups/(?:[A-Za-z0-9][A-Za-z0-9._-]*/)+[A-Za-z0-9][A-Za-z0-9._-]*\.sql$`)

// load sets what an update command loads: plugins run their own updaters (premium plugins), and
// for themes the active theme's updater too (premium themes).
func load(c Command, of Extensions) Command {
	c = c.WithPlugins()
	if of == ExtensionThemes {
		c = c.WithThemes()
	}
	return c
}

func extensions(of Extensions) error {
	if of != ExtensionPlugins && of != ExtensionThemes {
		return fmt.Errorf("extensions %q: %w", of, ErrBadArgument)
	}
	return nil
}

// UpdatePreview lists the plugins or themes an update would change, without changing them.
func UpdatePreview(of Extensions) Command {
	if err := extensions(of); err != nil {
		return Command{err: err}
	}
	return load(Command{args: []string{string(of), "update", "--all", "--dry-run", "--format=json"},
		skipPlugins: true, skipThemes: true}, of)
}

// UpdateOne updates one plugin or theme to the version its update offers.
func UpdateOne(of Extensions, slug string) Command {
	if err := extensions(of); err != nil {
		return Command{err: err}
	}
	if !slugName.MatchString(slug) {
		return Command{err: fmt.Errorf("%s %q: %w", of, slug, ErrBadArgument)}
	}
	return load(Command{args: []string{string(of), "update", slug, "--format=json"},
		skipPlugins: true, skipThemes: true}, of)
}

// UpdateCore updates WordPress to the version the preview showed.
func UpdateCore(to string) Command {
	if !version.MatchString(to) {
		return Command{err: fmt.Errorf("version %q: %w", to, ErrBadArgument)}
	}
	return Command{args: []string{"core", "update", "--version=" + to}, skipPlugins: true, skipThemes: true}
}

// UpdateDatabase runs WordPress's database upgrade, which a core update leaves to it; network
// upgrades every site of a multisite network.
func UpdateDatabase(network bool) Command {
	args := []string{"core", "update-db"}
	if network {
		args = append(args, "--network")
	}
	return Command{args: args, skipPlugins: true, skipThemes: true}
}

// UpdateLanguages updates the installed translations of WordPress, the plugins or the themes.
func UpdateLanguages(of Languages) Command {
	args := []string{"language", string(of), "update"}
	switch of {
	case LanguagesOfCore:
	case LanguagesOfPlugins, LanguagesOfThemes:
		args = append(args, "--all")
	default:
		return Command{err: fmt.Errorf("languages of %q: %w", of, ErrBadArgument)}
	}
	return Command{args: args, skipPlugins: true, skipThemes: true}
}

// Maintenance turns WordPress's maintenance mode on or off.
func Maintenance(on bool) Command {
	action := "deactivate"
	if on {
		action = "activate"
	}
	return Command{args: []string{"maintenance-mode", action}, skipPlugins: true, skipThemes: true}
}

// MaintenanceActive answers through its exit code: 0 when the maintenance mode is on.
func MaintenanceActive() Command {
	return Command{args: []string{"maintenance-mode", "is-active"}, skipPlugins: true, skipThemes: true}
}

// DatabaseExport dumps the database into a file of Vigie's backups folder; the credentials of
// wp-config.php never leave the server.
func DatabaseExport(file string) Command {
	if !backupFile.MatchString(file) {
		return Command{err: fmt.Errorf("backup file %q: %w", file, ErrBadArgument)}
	}
	return Command{args: []string{"db", "export", file, "--porcelain"}, skipPlugins: true, skipThemes: true}
}

// DatabaseImport loads a dump of Vigie's backups folder into the database.
func DatabaseImport(file string) Command {
	if !backupFile.MatchString(file) {
		return Command{err: fmt.Errorf("backup file %q: %w", file, ErrBadArgument)}
	}
	return Command{args: []string{"db", "import", file}, skipPlugins: true, skipThemes: true}
}

// ExtensionDir returns the folder of a plugin or theme; a theme can live in a plugin's folder, so
// the plugins are loaded for themes.
func ExtensionDir(of Extensions, slug string) Command {
	if err := extensions(of); err != nil {
		return Command{err: err}
	}
	if !slugName.MatchString(slug) {
		return Command{err: fmt.Errorf("%s %q: %w", of, slug, ErrBadArgument)}
	}
	c := Command{args: []string{string(of), "path", slug, "--dir"}, skipPlugins: true, skipThemes: true}
	if of == ExtensionThemes {
		c = c.WithPlugins()
	}
	return c
}

// ConfigConstant reads a constant of wp-config.php as JSON; an undefined one is an error.
func ConfigConstant(name string) Command {
	if !constantName.MatchString(name) {
		return Command{err: fmt.Errorf("constant %q: %w", name, ErrBadArgument)}
	}
	return Command{args: []string{"config", "get", name, "--type=constant", "--format=json"},
		skipPlugins: true, skipThemes: true}
}

// Pending is a plugin or theme an update would change.
type Pending struct {
	Name          string
	Version       string
	UpdateVersion string
}

// ParseUpdatePreview reads UpdatePreview's output. With --format=json, WP-CLI prints nothing when
// there is no update.
func ParseUpdatePreview(out []byte) ([]Pending, error) {
	pending := []Pending{}
	if len(bytes.TrimSpace(out)) == 0 {
		return pending, nil
	}
	var raw []struct {
		Name          string `json:"name"`
		Version       loose  `json:"version"`
		UpdateVersion loose  `json:"update_version"`
	}
	if err := decode(out, &raw); err != nil {
		return nil, fmt.Errorf("update preview: %w", err)
	}
	for _, r := range raw {
		if r.Name == "" || r.UpdateVersion == "" {
			return nil, fmt.Errorf("update preview: entry without name or version: %w", ErrUnexpectedOutput)
		}
		pending = append(pending, Pending{Name: r.Name, Version: string(r.Version), UpdateVersion: string(r.UpdateVersion)})
	}
	return pending, nil
}

// UpdateOutcome is how the update of one plugin or theme ended.
type UpdateOutcome string

const (
	UpdateDone    UpdateOutcome = "done"
	UpdateFailed  UpdateOutcome = "failed"  // WordPress could not install it; the old version stays
	UpdateNothing UpdateOutcome = "nothing" // there was no update to make
)

// Updated is the result of UpdateOne.
type Updated struct {
	Name     string
	From, To string
	Outcome  UpdateOutcome
	Message  string // WP-CLI's error, when it failed
}

// ParseUpdateOne reads UpdateOne's result: a JSON line per item, then exit 1 when one failed;
// nothing at all when there was no update. Any other failure means WordPress could not load.
func ParseUpdateOne(res sshx.Result) (Updated, error) {
	if len(bytes.TrimSpace(res.Stdout)) == 0 {
		if err := Check(res); err != nil {
			return Updated{}, err
		}
		return Updated{Outcome: UpdateNothing}, nil
	}
	var raw []struct {
		Name       string `json:"name"`
		OldVersion loose  `json:"old_version"`
		NewVersion loose  `json:"new_version"`
		Status     string `json:"status"`
	}
	if err := decode(res.Stdout, &raw); err != nil || len(raw) != 1 {
		return Updated{}, fmt.Errorf("update: %d items: %w", len(raw), errors.Join(err, ErrUnexpectedOutput))
	}
	r := raw[0]
	u := Updated{Name: r.Name, From: string(r.OldVersion), To: string(r.NewVersion)}
	switch {
	case r.Status == "Updated" && res.ExitCode == 0:
		u.Outcome = UpdateDone
	case r.Status == "Error":
		u.Outcome, u.Message = UpdateFailed, strings.TrimSpace(string(res.Stderr))
	default:
		return Updated{}, fmt.Errorf("update of %q: status %q, exit %d: %w", r.Name, r.Status, res.ExitCode, ErrUnexpectedOutput)
	}
	return u, nil
}

// Done reads the result of a command that changes a site and only says whether it worked.
func Done(res sshx.Result) error {
	if res.ExitCode == 0 {
		return nil
	}
	msg := strings.TrimSpace(string(res.Stderr))
	if msg == "" {
		msg = strings.TrimSpace(string(res.Stdout))
	}
	if msg == "" {
		msg = fmt.Sprintf("exit status %d", res.ExitCode)
	}
	return fmt.Errorf("%w: %s", ErrFailed, msg)
}

// MaintenanceSet reads Maintenance's result: already in the state asked is no failure, as
// WordPress removes the maintenance mode it sets during its own update.
func MaintenanceSet(res sshx.Result) error {
	msg := string(res.Stderr)
	if res.ExitCode != 0 && (strings.Contains(msg, "Maintenance mode already activated") || strings.Contains(msg, "Maintenance mode already deactivated")) {
		return nil
	}
	return Done(res)
}

// MaintenanceIsActive reads MaintenanceActive's exit code.
func MaintenanceIsActive(res sshx.Result) (bool, error) {
	switch {
	case res.ExitCode == 0:
		return true, nil
	case res.ExitCode == 1 && len(bytes.TrimSpace(res.Stderr)) == 0:
		return false, nil
	}
	return false, Check(res)
}

// ParseDatabaseExport reads DatabaseExport's result: the file written.
func ParseDatabaseExport(res sshx.Result) (string, error) {
	if res.ExitCode != 0 {
		msg := string(res.Stderr)
		// WP-CLI runs the tool through env, which says so when it is missing.
		if strings.Contains(msg, "No such file or directory") && (strings.Contains(msg, "mariadb-dump") || strings.Contains(msg, "mysqldump")) {
			return "", fmt.Errorf("%w: %s", ErrNoDumpTool, strings.TrimSpace(msg))
		}
		return "", Done(res)
	}
	file := strings.TrimSpace(string(res.Stdout))
	if !backupFile.MatchString(file) {
		return "", fmt.Errorf("database export %q: %w", file, ErrUnexpectedOutput)
	}
	return file, nil
}

// ParseDir reads ExtensionDir's output: one absolute, clean path.
func ParseDir(out []byte) (string, error) {
	dir := strings.TrimSpace(string(out))
	if !strings.HasPrefix(dir, "/") || strings.ContainsAny(dir, "\n") || path.Clean(dir) != dir {
		return "", fmt.Errorf("folder %q: %w", dir, ErrUnexpectedOutput)
	}
	return dir, nil
}

// DebugLog is where WordPress writes its PHP errors.
type DebugLog struct {
	Enabled bool
	Path    string // its own file; empty for debug.log in the content folder
}

// ParseDebugLog reads the constants WP_DEBUG and WP_DEBUG_LOG as wp_debug_mode() does: WordPress
// writes its log only when WP_DEBUG is true for PHP, to the content folder's debug.log when
// WP_DEBUG_LOG is "true" or "1" (in any case, or true, or 1), to the file it names when it is
// another string.
func ParseDebugLog(debug, log sshx.Result) (DebugLog, error) {
	on, _, err := ParseConstant(debug)
	if err != nil {
		return DebugLog{}, err
	}
	switch on.(type) {
	case bool, float64, string, nil:
	default:
		return DebugLog{}, fmt.Errorf("WP_DEBUG %v: %w", on, ErrUnexpectedOutput)
	}
	if !phpTrue(on) {
		return DebugLog{}, nil
	}
	value, _, err := ParseConstant(log)
	if err != nil {
		return DebugLog{}, err
	}
	switch v := value.(type) {
	case bool:
		return DebugLog{Enabled: v}, nil
	case float64:
		return DebugLog{Enabled: v == 1}, nil
	case string:
		if l := strings.ToLower(v); l == "true" || l == "1" {
			return DebugLog{Enabled: true}, nil
		}
		return DebugLog{Enabled: v != "", Path: v}, nil
	case nil:
		return DebugLog{}, nil
	}
	return DebugLog{}, fmt.Errorf("WP_DEBUG_LOG %v: %w", value, ErrUnexpectedOutput)
}

// phpTrue tells whether a constant's value is true for PHP: false, 0, "", "0" and null are not.
func phpTrue(v any) bool {
	switch v := v.(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case string:
		return v != "" && v != "0"
	}
	return v != nil
}

// ParseConstant reads ConfigConstant's result: its JSON value (a bool, string, number or nil), or
// not defined (nil, false).
func ParseConstant(res sshx.Result) (any, bool, error) {
	if res.ExitCode != 0 {
		if strings.Contains(string(res.Stderr), "is not defined") {
			return nil, false, nil
		}
		return nil, false, Check(res)
	}
	var value any
	if err := json.Unmarshal(bytes.TrimSpace(res.Stdout), &value); err != nil {
		return nil, false, fmt.Errorf("constant: %w: %v", ErrUnexpectedOutput, err)
	}
	switch value.(type) {
	case bool, string, float64, nil:
		return value, true, nil
	}
	return nil, false, fmt.Errorf("constant %s: %w", bytes.TrimSpace(res.Stdout), ErrUnexpectedOutput)
}
