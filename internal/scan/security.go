package scan

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/MarJC5/vigie/internal/lookup"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// The checks a finding of kind model.KindCheck names.
const (
	checkCoreFiles       = "core-files"
	checkPluginFiles     = "plugin-files"
	checkUploads         = "uploads"
	checkAdministrators  = "administrators"
	checkSettings        = "settings"
	checkCron            = "cron"
	checkAutoload        = "autoload"
	checkDatabaseVersion = "database-version"
	checkVulnerabilities = "vulnerabilities"
	checkWordPressOrg    = "wordpress.org"
	checkCertificate     = "certificate"
	checkDomain          = "domain"
	checkDebugLog        = "debug-log"
)

// fileChecks are the checks of the files' comparison, carried over with it.
var fileChecks = []string{checkCoreFiles, checkPluginFiles, checkUploads}

// fromFiles tells whether a finding comes from the comparison of the files.
func fromFiles(f model.Finding) bool {
	switch f.Kind {
	case model.KindCoreFile, model.KindPluginFile, model.KindPluginUnverified, model.KindUploadsPHP, model.KindUploadsPHPMore:
		return true
	case model.KindCheck:
		return slices.Contains(fileChecks, f.Identifier)
	}
	return false
}

// stops tells whether a security read's error ends the scan: a lost connection or a cancel. Any
// other failure leaves that check "not checked" and the scan goes on.
func stops(ctx context.Context, err error) bool {
	var conn *sshx.ConnectionError
	return ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.As(err, &conn)
}

// notChecked is the finding of a check that could not run.
func notChecked(check string, err error) model.Finding {
	facts := map[string]string{"reason": "error", "error": err.Error()}
	if errors.Is(err, lookup.ErrOff) {
		facts = map[string]string{"reason": "off"}
	}
	return model.Finding{Kind: model.KindCheck, Subject: "site", Identifier: check, Level: model.LevelNotChecked, Facts: facts}
}

// security reads on the server what a security scan needs, after the versions and updates.
func (r *run) security(ctx context.Context, result *model.Scan) error {
	for _, read := range []func(context.Context, *model.Scan) error{r.files, r.administrators, r.settings, r.database} {
		if err := read(ctx, result); err != nil {
			return err
		}
	}
	return nil
}

// filesDue tells whether the files must be compared again: asked, never done, done more than a
// day ago, failed, or WordPress or a plugin changed version since.
func (r *run) filesDue(result *model.Scan) bool {
	prev := r.job.Previous
	if r.job.CheckFiles || prev.FilesChecked.IsZero() || r.scanner.Now().Sub(prev.FilesChecked) >= filesEvery ||
		prev.WordPress != result.WordPress {
		return true
	}
	if slices.ContainsFunc(prev.Findings, func(f model.Finding) bool { return f.Kind == model.KindCheck && fromFiles(f) }) {
		return true
	}
	versions := map[string]string{}
	for _, i := range prev.Items {
		if i.Kind == model.ItemPlugin {
			versions[i.Slug] = i.Version
		}
	}
	for _, i := range result.Items {
		if v, ok := versions[i.Slug]; i.Kind == model.ItemPlugin && (!ok || v != i.Version) {
			return true
		}
	}
	return false
}

// files compares WordPress's and the plugins' files with wordpress.org's, and lists PHP files in
// the uploads, at most once a day: in between, the last comparison is carried over.
func (r *run) files(ctx context.Context, result *model.Scan) error {
	if !r.filesDue(result) {
		result.FilesChecked = r.job.Previous.FilesChecked
		for _, f := range r.job.Previous.Findings {
			if fromFiles(f) {
				result.Findings = append(result.Findings, f)
			}
		}
		return nil
	}
	r.total += filesSteps
	result.FilesChecked = r.scanner.Now()
	versions := map[string]string{}
	for _, i := range result.Items {
		if i.Kind == model.ItemPlugin {
			versions[i.Slug] = i.Version
		}
	}

	res, err := r.exec(ctx, wpcli.CoreVerifyChecksums(), r.job.Site.Path)
	if err != nil {
		return err
	}
	core, err := wpcli.ParseCoreChecksums(res)
	if err != nil {
		result.Findings = append(result.Findings, notChecked(checkCoreFiles, err))
	}
	for _, p := range core {
		result.Findings = append(result.Findings, model.Finding{Kind: model.KindCoreFile, Subject: "core", Identifier: p.File,
			Version: result.WordPress, Level: fileLevel(""), Facts: map[string]string{"change": string(p.Change)}})
	}

	if res, err = r.exec(ctx, wpcli.PluginVerifyChecksums(), r.job.Site.Path); err != nil {
		return err
	}
	plugins, err := wpcli.ParsePluginChecksums(res)
	if err != nil {
		result.Findings = append(result.Findings, notChecked(checkPluginFiles, err))
	}
	for _, p := range plugins.Problems {
		result.Findings = append(result.Findings, model.Finding{Kind: model.KindPluginFile, Subject: "plugin:" + p.Plugin,
			Identifier: p.File, Version: versions[p.Plugin], Level: fileLevel(p.Plugin),
			Facts: map[string]string{"change": string(p.Change)}})
	}
	for _, slug := range plugins.Skipped {
		result.Findings = append(result.Findings, model.Finding{Kind: model.KindPluginUnverified, Subject: "plugin:" + slug,
			Version: versions[slug], Level: model.LevelInfo})
	}
	return r.uploads(ctx, result)
}

// uploads lists PHP files under the uploads folder of the site's real folder.
func (r *run) uploads(ctx context.Context, result *model.Scan) error {
	root, err := r.shell(ctx, func() (string, error) { return remote.RealRoot(r.job.Site.Path) })
	if err == nil {
		var dir string
		if dir, err = wpcli.ParseDir(root); err == nil {
			var out []byte
			if out, err = r.shell(ctx, func() (string, error) { return remote.UploadsPHP(dir) }); err == nil {
				files, more := remote.ParseUploadsPHP(dir, out)
				for _, f := range files {
					result.Findings = append(result.Findings, model.Finding{Kind: model.KindUploadsPHP, Subject: "site",
						Identifier: f, Level: model.LevelUrgent})
				}
				if more {
					result.Findings = append(result.Findings, model.Finding{Kind: model.KindUploadsPHPMore, Subject: "site",
						Level: model.LevelUrgent, Facts: map[string]string{"shown": strconv.Itoa(remote.UploadsShown)}})
				}
				return nil
			}
		}
	}
	if stops(ctx, err) {
		return err
	}
	result.Findings = append(result.Findings, notChecked(checkUploads, err))
	return nil
}

// wordfence are the slugs of the plugins that bring Wordfence's two-factor.
var wordfence = []string{"wordfence", "wordfence-login-security"}

// administrators lists the administrators (and the super administrators of a network) with their
// two-factor: what Two Factor, Solid Security and WP 2FA store in the user's meta, and Wordfence's
// table when Wordfence is active.
func (r *run) administrators(ctx context.Context, result *model.Scan) error {
	out, err := r.read(ctx, wpcli.Administrators())
	var admins []wpcli.Account
	if err == nil {
		admins, err = wpcli.ParseAdministrators(out)
	}
	if err != nil {
		if stops(ctx, err) {
			return err
		}
		result.Findings = append(result.Findings, notChecked(checkAdministrators, err))
		return nil
	}
	r.total += len(admins)

	supers := map[string]bool{}
	if r.job.Site.Multisite {
		res, err := r.exec(ctx, wpcli.SuperAdmins(), r.job.Site.Path)
		if err != nil {
			return err
		}
		logins, err := wpcli.ParseSuperAdmins(res)
		if err != nil {
			result.Findings = append(result.Findings, notChecked(checkAdministrators, err))
		}
		for _, l := range logins {
			supers[l] = true
		}
	}

	withWordfence, wordfenceKnown, err := r.wordfenceUsers(ctx, result)
	if err != nil {
		return err
	}
	for _, a := range admins {
		out, err := r.read(ctx, wpcli.TwoFactorMeta(a.ID))
		var tf wpcli.TwoFactor
		if err == nil {
			tf, err = wpcli.ParseTwoFactorMeta(out)
		}
		if err != nil && stops(ctx, err) {
			return err
		}
		state := twoFactorOff
		switch {
		case err == nil && (len(tf.Providers) > 0 || tf.WP2FA != "") || slices.Contains(withWordfence, a.ID):
			state = twoFactorOn
		case err != nil || !wordfenceKnown:
			state = twoFactorUnknown
		}
		facts := map[string]string{"email": a.Email, "registered": a.Registered.Format(time.DateOnly), "two-factor": state}
		if supers[a.Login] {
			facts["super"] = "true"
			delete(supers, a.Login)
		}
		result.Findings = append(result.Findings, model.Finding{Kind: model.KindAdministrator, Subject: "user:" + a.Login,
			Level: twoFactorLevel(state), Facts: facts})
	}
	// Super administrators who are not administrators of the main site: their meta is not read.
	for _, login := range slices.Sorted(maps.Keys(supers)) {
		result.Findings = append(result.Findings, model.Finding{Kind: model.KindAdministrator, Subject: "user:" + login,
			Level: twoFactorLevel(twoFactorUnknown), Facts: map[string]string{"two-factor": twoFactorUnknown, "super": "true"}})
	}
	return nil
}

// wordfenceUsers reads the IDs of the users with Wordfence's two-factor when Wordfence is active;
// known is false when its table could not be read (no mysql client on the server).
func (r *run) wordfenceUsers(ctx context.Context, result *model.Scan) (ids []int, known bool, err error) {
	if !slices.ContainsFunc(result.Items, func(i model.ScanItem) bool {
		return i.Kind == model.ItemPlugin && slices.Contains(wordfence, i.Slug) && strings.HasPrefix(i.Status, "active")
	}) {
		return nil, true, nil
	}
	r.total += 2
	out, err := r.read(ctx, wpcli.DatabasePrefix())
	var prefix string
	if err == nil {
		prefix, err = wpcli.ParseDatabasePrefix(out)
	}
	if err == nil {
		var res sshx.Result
		if res, err = r.exec(ctx, wpcli.WordfenceTwoFactorUsers(prefix), r.job.Site.Path); err == nil {
			ids, err = wpcli.ParseUserIDs(res)
		}
	}
	if err != nil {
		if stops(ctx, err) {
			return nil, false, err
		}
		return nil, false, nil
	}
	return ids, true, nil
}

// settings reads wp-config.php's constants, the options and WP-Cron.
func (r *run) settings(ctx context.Context, result *model.Scan) error {
	if err := r.constants(ctx, result); err != nil {
		return err
	}
	res, err := r.exec(ctx, wpcli.OptionGet("blog_public"), r.job.Site.Path)
	if err != nil {
		return err
	}
	switch public, ok, err := wpcli.ParseOption(res); {
	case err != nil:
		result.Findings = append(result.Findings, notChecked(checkSettings, err))
	case ok && public == "0":
		result.Findings = append(result.Findings, model.Finding{Kind: model.KindHiddenFromSearch, Subject: "site", Level: model.LevelAttention})
	}
	if strings.HasPrefix(result.Home, "http://") {
		result.Findings = append(result.Findings, model.Finding{Kind: model.KindNoHTTPS, Subject: "site", Level: model.LevelAttention})
	}
	if err := r.cron(ctx, result); err != nil {
		return err
	}
	return r.autoload(ctx, result)
}

// The constants of wp-config.php the rules read; only those defined are asked.
var constants = []string{"WP_DEBUG", "WP_DEBUG_DISPLAY", "DISALLOW_FILE_EDIT"}

// constants reads whether files can be edited from the administration, and whether PHP errors
// show on the pages (WP_DEBUG, with WP_DEBUG_DISPLAY true by default).
func (r *run) constants(ctx context.Context, result *model.Scan) error {
	out, err := r.read(ctx, wpcli.ConfigNames())
	var names []string
	if err == nil {
		names, err = wpcli.ParseConfigNames(out)
	}
	values := map[string]any{}
	for _, name := range constants {
		if err != nil || !slices.Contains(names, name) {
			continue
		}
		r.total++
		var res sshx.Result
		if res, err = r.exec(ctx, wpcli.ConfigConstant(name), r.job.Site.Path); err == nil {
			values[name], _, err = wpcli.ParseConstant(res)
		}
	}
	if err != nil {
		if stops(ctx, err) {
			return err
		}
		result.Findings = append(result.Findings, notChecked(checkSettings, err))
		return nil
	}
	if v, ok := values["DISALLOW_FILE_EDIT"]; !ok || !phpTrue(v) {
		result.Findings = append(result.Findings, model.Finding{Kind: model.KindFileEditing, Subject: "site", Level: model.LevelAttention})
	}
	if display, ok := values["WP_DEBUG_DISPLAY"]; phpTrue(values["WP_DEBUG"]) && (!ok || phpTrue(display)) {
		result.Findings = append(result.Findings, model.Finding{Kind: model.KindErrorsDisplayed, Subject: "site", Level: model.LevelAttention})
	}
	return nil
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

// cron reports the WP-Cron events overdue by more than an hour: WP-Cron does not run.
func (r *run) cron(ctx context.Context, result *model.Scan) error {
	out, err := r.read(ctx, wpcli.CronEvents())
	var events []wpcli.CronEvent
	if err == nil {
		events, err = wpcli.ParseCronEvents(out)
	}
	if err != nil {
		if stops(ctx, err) {
			return err
		}
		result.Findings = append(result.Findings, notChecked(checkCron, err))
		return nil
	}
	late := r.scanner.Now().Add(-cronLate)
	var overdue []wpcli.CronEvent
	for _, e := range events {
		if e.Next.Before(late) {
			overdue = append(overdue, e)
		}
	}
	if len(overdue) == 0 {
		return nil
	}
	oldest := slices.MinFunc(overdue, func(a, b wpcli.CronEvent) int { return a.Next.Compare(b.Next) })
	result.Findings = append(result.Findings, model.Finding{Kind: model.KindCronOverdue, Subject: "site", Level: model.LevelAttention,
		Facts: map[string]string{"count": strconv.Itoa(len(overdue)), "hook": oldest.Hook, "since": oldest.Next.Format(time.RFC3339)}})
	return nil
}

// autoload adds up the options loaded on every page.
func (r *run) autoload(ctx context.Context, result *model.Scan) error {
	options, err := r.read(ctx, wpcli.OptionSizes(false))
	var size int64
	if err == nil {
		var transients []byte
		if transients, err = r.read(ctx, wpcli.OptionSizes(true)); err == nil {
			size, err = wpcli.ParseAutoloadedSize(options, transients)
		}
	}
	if err != nil {
		if stops(ctx, err) {
			return err
		}
		result.Findings = append(result.Findings, notChecked(checkAutoload, err))
		return nil
	}
	if size > autoloadLimit {
		result.Findings = append(result.Findings, model.Finding{Kind: model.KindAutoloadSize, Subject: "site", Level: model.LevelAttention,
			Facts: map[string]string{"bytes": strconv.FormatInt(size, 10), "limit": strconv.Itoa(autoloadLimit)}})
	}
	return nil
}

// database asks the database server its version, which needs the mysql client on the server.
func (r *run) database(ctx context.Context, result *model.Scan) error {
	res, err := r.exec(ctx, wpcli.DatabaseVersion(), r.job.Site.Path)
	if err != nil {
		return err
	}
	v, err := wpcli.ParseDatabaseVersion(res)
	if err != nil {
		result.Findings = append(result.Findings, notChecked(checkDatabaseVersion, err))
		return nil
	}
	result.Findings = append(result.Findings, model.Finding{Kind: model.KindDatabaseVersion, Subject: "site", Level: model.LevelInfo,
		Facts: map[string]string{"version": v}})
	return nil
}
