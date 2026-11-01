package wpcli

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
)

// CoreVerifyChecksums compares the WordPress files with the checksums wordpress.org publishes for
// the installed version and locale; the server fetches them. It runs before WordPress loads.
func CoreVerifyChecksums() Command {
	return Command{args: []string{"core", "verify-checksums"}, skipPlugins: true, skipThemes: true}
}

// PluginVerifyChecksums compares the files of every plugin wordpress.org knows with its published
// checksums; the others are skipped.
func PluginVerifyChecksums() Command {
	return Command{args: []string{"plugin", "verify-checksums", "--all", "--format=json"}, skipPlugins: true, skipThemes: true}
}

// ErrNotVerified is returned when a checksum command could not compare anything, for example when
// the server could not fetch the checksums: the files are not checked.
var ErrNotVerified = errors.New("files not verified")

// FileChange is how a file differs from the published one.
type FileChange string

const (
	FileChanged FileChange = "changed"
	FileMissing FileChange = "missing"
	FileAdded   FileChange = "added"
)

// FileProblem is a file that differs from the published version: in WordPress, or in a plugin.
type FileProblem struct {
	Plugin string // empty for WordPress's own files
	File   string // relative to WordPress's or the plugin's folder
	Change FileChange
}

// coreWarnings are WP-CLI's warnings about WordPress files, one per file (checksum-command 2.3).
var coreWarnings = []struct {
	prefix string
	change FileChange
}{
	{"Warning: File doesn't exist: ", FileMissing},
	{"Warning: File doesn't verify against checksum: ", FileChanged},
	{"Warning: File should not exist: ", FileAdded},
}

// ParseCoreChecksums reads `core verify-checksums`. Its warnings are the result: an added file
// alone still ends with "Success", so the warnings are read whatever the exit code.
func ParseCoreChecksums(res sshx.Result) ([]FileProblem, error) {
	var problems []FileProblem
	var failure string
	for _, line := range strings.Split(string(res.Stderr), "\n") {
		line = strings.TrimSpace(line)
		for _, w := range coreWarnings {
			if file, ok := strings.CutPrefix(line, w.prefix); ok {
				problems = append(problems, FileProblem{File: file, Change: w.change})
			}
		}
		if msg, ok := strings.CutPrefix(line, "Error: "); ok {
			failure = msg
		}
	}
	switch {
	case res.ExitCode == 0 && strings.Contains(string(res.Stdout), "Success: WordPress installation verifies against checksums."):
		return problems, nil
	case res.ExitCode != 0 && failure == "WordPress installation doesn't verify against checksums.":
		return problems, nil
	case res.ExitCode != 0 && failure != "":
		return nil, fmt.Errorf("%w: %s", ErrNotVerified, failure)
	}
	return nil, fmt.Errorf("core verify-checksums: %w", ErrUnexpectedOutput)
}

// PluginChecksums is the result of `plugin verify-checksums --all`: the files that differ, and how
// many plugins were verified; the skipped ones (unknown to wordpress.org) are not.
type PluginChecksums struct {
	Problems []FileProblem
	Total    int
	Verified int
	Failed   int
	Skipped  []string // their slugs
}

var (
	pluginsSummary = regexp.MustCompile(`(?:Verified|Only verified) (\d+) of (\d+) plugins(?: \(([^)]*)\))?\.`)
	noneVerified   = regexp.MustCompile(`No plugins verified(?: \(([^)]*)\))?\.`)
	summaryCount   = regexp.MustCompile(`(\d+) (failed|skipped)`)
	pluginSkipped  = regexp.MustCompile(`^Warning: Could not retrieve the checksums for version \S+ of plugin (\S+), skipping\.$`)
)

// fileMessages are the plugin command's messages, by file.
var fileMessages = map[string]FileChange{
	"File was added":          FileAdded,
	"Checksum does not match": FileChanged,
}

// ParsePluginChecksums reads `plugin verify-checksums --all --format=json`: the files on stdout
// (only when some differ), the skipped plugins in the warnings, the counts in the last line.
func ParsePluginChecksums(res sshx.Result) (PluginChecksums, error) {
	var out PluginChecksums
	summary := false
	for _, line := range strings.Split(string(res.Stdout)+"\n"+string(res.Stderr), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "["):
			var files []struct {
				Plugin  string `json:"plugin_name"`
				File    string `json:"file"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal([]byte(line), &files); err != nil {
				return PluginChecksums{}, fmt.Errorf("plugin verify-checksums: %w: %w", ErrUnexpectedOutput, err)
			}
			for _, f := range files {
				change, ok := fileMessages[f.Message]
				if !ok {
					change = FileChanged
				}
				out.Problems = append(out.Problems, FileProblem{Plugin: f.Plugin, File: f.File, Change: change})
			}
		case pluginSkipped.MatchString(line):
			out.Skipped = append(out.Skipped, pluginSkipped.FindStringSubmatch(line)[1])
		case pluginsSummary.MatchString(line):
			m := pluginsSummary.FindStringSubmatch(line)
			out.Verified, _ = strconv.Atoi(m[1])
			out.Total, _ = strconv.Atoi(m[2])
			out.Failed, summary = failedOf(m[3]), true
		case noneVerified.MatchString(line):
			m := noneVerified.FindStringSubmatch(line)
			out.Failed, summary = failedOf(m[1]), true
			out.Total = out.Failed + len(out.Skipped)
		}
	}
	if !summary {
		return PluginChecksums{}, fmt.Errorf("plugin verify-checksums: %w", ErrUnexpectedOutput)
	}
	return out, nil
}

// failedOf reads "1 failed, 1 skipped".
func failedOf(counts string) int {
	for _, m := range summaryCount.FindAllStringSubmatch(counts, -1) {
		if m[2] == "failed" {
			n, _ := strconv.Atoi(m[1])
			return n
		}
	}
	return 0
}

// Administrators lists the administrators of the site (of the main site of a network).
func Administrators() Command {
	return Command{args: []string{"user", "list", "--role=administrator",
		"--fields=ID,user_login,user_email,user_registered", "--format=json"}, skipPlugins: true, skipThemes: true}
}

// SuperAdmins lists the super administrators of a network.
func SuperAdmins() Command {
	return Command{args: []string{"super-admin", "list", "--format=json"}, skipPlugins: true, skipThemes: true}
}

// twoFactorKeys are where Two Factor, Solid Security and WP 2FA record the methods a user turned
// on. Never their secrets (TOTP keys, backup codes): Vigie does not read them.
const twoFactorKeys = "_two_factor_enabled_providers,wp_2fa_enabled_methods"

// TwoFactorMeta reads which two-factor methods a user turned on, by user ID.
func TwoFactorMeta(userID int) Command {
	if userID <= 0 {
		return Command{err: fmt.Errorf("user %d: %w", userID, ErrBadArgument)}
	}
	return Command{args: []string{"user", "meta", "list", strconv.Itoa(userID), "--keys=" + twoFactorKeys,
		"--format=json", "--unserialize"}, skipPlugins: true, skipThemes: true}
}

// DatabasePrefix answers the prefix of the site's tables.
func DatabasePrefix() Command {
	return Command{args: []string{"db", "prefix"}, skipPlugins: true, skipThemes: true}
}

var tablePrefix = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)

// WordfenceTwoFactorUsers lists the IDs of the users with Wordfence two-factor: its table holds no
// user meta. Only the user_id column is read, never the secrets. It needs the mysql client on the
// server.
func WordfenceTwoFactorUsers(prefix string) Command {
	if !tablePrefix.MatchString(prefix) {
		return Command{err: fmt.Errorf("table prefix %q: %w", prefix, ErrBadArgument)}
	}
	return Command{args: []string{"db", "query", "SELECT DISTINCT user_id FROM `" + prefix + "wfls_2fa_secrets`",
		"--skip-column-names"}, skipPlugins: true, skipThemes: true}
}

// Account is a user with a role worth watching.
type Account struct {
	ID         int
	Login      string
	Email      string
	Registered time.Time // UTC, as WordPress stores it
}

// ParseAdministrators reads Administrators' output.
func ParseAdministrators(out []byte) ([]Account, error) {
	var users []struct {
		ID         int    `json:"ID"`
		Login      string `json:"user_login"`
		Email      string `json:"user_email"`
		Registered string `json:"user_registered"`
	}
	if err := decode(out, &users); err != nil {
		return nil, fmt.Errorf("user list: %w", err)
	}
	accounts := make([]Account, len(users))
	for i, u := range users {
		accounts[i] = Account{ID: u.ID, Login: u.Login, Email: u.Email}
		if u.Registered != "" && u.Registered != "0000-00-00 00:00:00" {
			at, err := time.ParseInLocation(time.DateTime, u.Registered, time.UTC)
			if err != nil {
				return nil, fmt.Errorf("user %s registered %q: %w", u.Login, u.Registered, ErrUnexpectedOutput)
			}
			accounts[i].Registered = at
		}
	}
	return accounts, nil
}

// ParseSuperAdmins reads SuperAdmins' output: the logins. A site that is not a network fails.
func ParseSuperAdmins(res sshx.Result) ([]string, error) {
	if err := Check(res); err != nil {
		return nil, err
	}
	var admins []struct {
		Login string `json:"user_login"`
	}
	if err := decode(res.Stdout, &admins); err != nil {
		return nil, fmt.Errorf("super-admin list: %w", err)
	}
	logins := make([]string, len(admins))
	for i, a := range admins {
		logins[i] = a.Login
	}
	return logins, nil
}

// TwoFactor is what a user turned on: Two Factor's (and Solid Security's) providers, such as
// "Two_Factor_Totp", and WP 2FA's method, such as "totp".
type TwoFactor struct {
	Providers []string
	WP2FA     string
}

// ParseTwoFactorMeta reads TwoFactorMeta's output.
func ParseTwoFactorMeta(out []byte) (TwoFactor, error) {
	var meta []struct {
		Key   string          `json:"meta_key"`
		Value json.RawMessage `json:"meta_value"`
	}
	if err := decode(out, &meta); err != nil {
		return TwoFactor{}, fmt.Errorf("user meta list: %w", err)
	}
	var tf TwoFactor
	for _, m := range meta {
		switch m.Key {
		case "_two_factor_enabled_providers":
			// An array once unserialized; a string when it was not stored as one.
			var list []string
			if err := json.Unmarshal(m.Value, &list); err != nil {
				var one string
				if err := json.Unmarshal(m.Value, &one); err != nil {
					return TwoFactor{}, fmt.Errorf("two-factor providers: %w", ErrUnexpectedOutput)
				}
				list = []string{one}
			}
			tf.Providers = append(tf.Providers, list...)
		case "wp_2fa_enabled_methods":
			if err := json.Unmarshal(m.Value, &tf.WP2FA); err != nil {
				return TwoFactor{}, fmt.Errorf("WP 2FA method: %w", ErrUnexpectedOutput)
			}
		}
	}
	return tf, nil
}

// ParseDatabasePrefix reads DatabasePrefix's output.
func ParseDatabasePrefix(out []byte) (string, error) {
	prefix := strings.TrimSpace(string(out))
	if !tablePrefix.MatchString(prefix) {
		return "", fmt.Errorf("db prefix %q: %w", prefix, ErrUnexpectedOutput)
	}
	return prefix, nil
}

// ParseUserIDs reads WordfenceTwoFactorUsers' output. A missing table means Wordfence is not
// there; any other failure (no mysql client) leaves it not verified.
func ParseUserIDs(res sshx.Result) ([]int, error) {
	if res.ExitCode != 0 {
		if strings.Contains(string(res.Stderr), "ERROR 1146") {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: %s", ErrNotVerified, strings.TrimSpace(string(res.Stderr)))
	}
	var ids []int
	for _, line := range strings.Fields(string(res.Stdout)) {
		id, err := strconv.Atoi(line)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("user ID %q: %w", line, ErrUnexpectedOutput)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// ConfigNames lists the names of the constants and variables wp-config.php defines, never their
// values: it also holds the database password and the salts.
func ConfigNames() Command {
	return Command{args: []string{"config", "list", "--fields=name,type", "--format=json"}, skipPlugins: true, skipThemes: true}
}

// OptionSizes lists every option with its autoload value and size, without the values; transients
// lists the transients instead, which WP-CLI leaves out otherwise.
func OptionSizes(transients bool) Command {
	args := []string{"option", "list", "--fields=option_name,autoload,size_bytes", "--format=json"}
	if transients {
		args = append(args, "--transients")
	}
	return Command{args: args, skipPlugins: true, skipThemes: true}
}

// CronEvents lists the scheduled WP-Cron events and when each is due. Not `cron test`: it runs the
// events due.
func CronEvents() Command {
	return Command{args: []string{"cron", "event", "list", "--fields=hook,next_run_gmt", "--format=json"},
		skipPlugins: true, skipThemes: true}
}

// DatabaseVersion asks the database server its version; it needs the mysql client on the server.
func DatabaseVersion() Command {
	return Command{args: []string{"db", "query", "SELECT VERSION()", "--skip-column-names"}, skipPlugins: true, skipThemes: true}
}

// ParseConfigNames reads ConfigNames' output: the names of the constants wp-config.php defines.
func ParseConfigNames(out []byte) ([]string, error) {
	var entries []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := decode(out, &entries); err != nil {
		return nil, fmt.Errorf("config list: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.Type == "constant" {
			names = append(names, e.Name)
		}
	}
	return names, nil
}

// ParseOption reads OptionGet's result: the value, or not set (an option that does not exist,
// whose default applies).
func ParseOption(res sshx.Result) (string, bool, error) {
	if res.ExitCode != 0 {
		if strings.Contains(string(res.Stderr), "Could not get '") {
			return "", false, nil
		}
		return "", false, Check(res)
	}
	return strings.TrimSpace(string(res.Stdout)), true, nil
}

// autoloaded are the autoload values WordPress loads on every page (wp_autoload_values_to_autoload,
// WordPress 6.6 and later; "yes" is the older value).
var autoloaded = map[string]bool{"yes": true, "on": true, "auto-on": true, "auto": true}

// ParseAutoloadedSize adds up the sizes of the options and transients loaded on every page, as
// Site Health does; WP-CLI's own total misses "auto" and "auto-on".
func ParseAutoloadedSize(options, transients []byte) (int64, error) {
	var total int64
	for _, out := range [][]byte{options, transients} {
		var list []struct {
			Autoload string          `json:"autoload"`
			Size     json.RawMessage `json:"size_bytes"`
		}
		if err := decode(out, &list); err != nil {
			return 0, fmt.Errorf("option list: %w", err)
		}
		for _, o := range list {
			if !autoloaded[o.Autoload] {
				continue
			}
			// A string in WP-CLI 2.12, perhaps a number elsewhere.
			n, err := strconv.ParseInt(strings.Trim(string(o.Size), `"`), 10, 64)
			if err != nil {
				return 0, fmt.Errorf("option size %s: %w", o.Size, ErrUnexpectedOutput)
			}
			total += n
		}
	}
	return total, nil
}

// CronEvent is a scheduled WP-Cron event.
type CronEvent struct {
	Hook string
	Next time.Time // UTC
}

// ParseCronEvents reads CronEvents' output.
func ParseCronEvents(out []byte) ([]CronEvent, error) {
	var list []struct {
		Hook string `json:"hook"`
		Next string `json:"next_run_gmt"`
	}
	if err := decode(out, &list); err != nil {
		return nil, fmt.Errorf("cron event list: %w", err)
	}
	events := make([]CronEvent, len(list))
	for i, e := range list {
		at, err := time.ParseInLocation(time.DateTime, e.Next, time.UTC)
		if err != nil {
			return nil, fmt.Errorf("cron event %s at %q: %w", e.Hook, e.Next, ErrUnexpectedOutput)
		}
		events[i] = CronEvent{Hook: e.Hook, Next: at}
	}
	return events, nil
}

// ParseDatabaseVersion reads DatabaseVersion's result; without the mysql client, not verified.
func ParseDatabaseVersion(res sshx.Result) (string, error) {
	if res.ExitCode != 0 {
		return "", fmt.Errorf("%w: %s", ErrNotVerified, strings.TrimSpace(string(res.Stderr)))
	}
	v := strings.TrimSpace(string(res.Stdout))
	if v == "" || strings.ContainsAny(v, "\n\t") {
		return "", fmt.Errorf("database version %q: %w", v, ErrUnexpectedOutput)
	}
	return v, nil
}

// PHPChecks lists the dashboard's checks of the PHP version serving it, which WordPress keeps a
// week as the site transient php_check_<md5 of PHP_VERSION> (wp_check_php_version, ADR-0028): the
// keys and their expiries, not the answers. The database only: WP-CLI cannot list an external
// object cache, and says so.
func PHPChecks() Command {
	return Command{args: []string{"transient", "list", "--network", "--search=php_check_*", "--fields=name,expiration", "--format=json"},
		skipPlugins: true, skipThemes: true}
}

// PHPCheck is one of the dashboard's checks: the md5 of the PHP version it was made with, and
// when WordPress lets it expire (zero when it never does).
type PHPCheck struct {
	Hash    string
	Expires time.Time // UTC
}

// ParsePHPChecks reads PHPChecks' result, and whether the transients are in an external object
// cache, where WP-CLI does not list them.
func ParsePHPChecks(res sshx.Result) ([]PHPCheck, bool, error) {
	if err := Check(res); err != nil {
		return nil, false, err
	}
	cached := strings.Contains(string(res.Stderr), "external object cache")
	var list []struct {
		Name       string          `json:"name"`
		Expiration json.RawMessage `json:"expiration"` // a number in WP-CLI 2.12, "false" when it never expires
	}
	if err := decode(res.Stdout, &list); err != nil {
		return nil, false, fmt.Errorf("transient list: %w", err)
	}
	checks := []PHPCheck{}
	for _, e := range list {
		hash, ok := strings.CutPrefix(e.Name, "php_check_")
		if !ok {
			return nil, false, fmt.Errorf("transient %q: %w", e.Name, ErrUnexpectedOutput)
		}
		c := PHPCheck{Hash: hash}
		if expiration := strings.Trim(string(e.Expiration), `"`); expiration != "false" {
			at, err := strconv.ParseInt(expiration, 10, 64)
			if err != nil {
				return nil, false, fmt.Errorf("transient %s expires %q: %w", e.Name, e.Expiration, ErrUnexpectedOutput)
			}
			c.Expires = time.Unix(at, 0).UTC()
		}
		checks = append(checks, c)
	}
	return checks, cached, nil
}

// releases are the PHP versions a dashboard's check can be named by, by the md5 WordPress keys it
// with: every x.y.z of PHP 5.6, 7 and 8, beyond the releases so far.
var releases = sync.OnceValue(func() map[string]string {
	m := map[string]string{}
	add := func(major, minor int) {
		for patch := range 100 {
			v := fmt.Sprintf("%d.%d.%d", major, minor, patch)
			m[md5Hex(v)] = v
		}
	}
	add(5, 6)
	for minor := range 5 {
		add(7, minor)
	}
	for minor := range 10 {
		add(8, minor)
	}
	return m
})

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// week is how long WordPress keeps a check of the PHP version.
const week = 7 * 24 * time.Hour

// WebPHP names the PHP version of the dashboard's newest check still kept: the command line's
// exact version string, or a release x.y.z (ADR-0028). A check of a version that is neither (a
// distribution's build that is not the command line's) says the web differs, unnamed.
func WebPHP(checks []PHPCheck, cached bool, cli string, now time.Time) model.PHPWeb {
	// A dated check is newer than one kept forever, which a cache plugin may leave.
	newer := func(a, b PHPCheck) bool {
		if a.Expires.IsZero() || b.Expires.IsZero() {
			return b.Expires.IsZero() && !a.Expires.IsZero()
		}
		return a.Expires.After(b.Expires)
	}
	var newest *PHPCheck
	for i, c := range checks {
		if !c.Expires.IsZero() && !c.Expires.After(now) {
			continue // expired: left until WordPress deletes it
		}
		if newest == nil || newer(c, *newest) {
			newest = &checks[i]
		}
	}
	switch {
	case newest == nil && cached:
		return model.PHPWeb{Unknown: model.PHPWebObjectCache}
	case newest == nil:
		return model.PHPWeb{Unknown: model.PHPWebNotNoted}
	}
	web := model.PHPWeb{}
	if !newest.Expires.IsZero() {
		web.Seen = newest.Expires.Add(-week)
	}
	switch {
	case cli != "" && newest.Hash == md5Hex(cli):
		web.Version = cli
	case releases()[newest.Hash] != "":
		web.Version = releases()[newest.Hash]
	default:
		return model.PHPWeb{Unknown: model.PHPWebUnmatched}
	}
	return web
}
