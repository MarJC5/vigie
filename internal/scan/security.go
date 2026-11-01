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
	checkPHPWeb          = "php-web"
	checkDebugLogFile    = "debug-log-file"
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
	result.Findings = append(result.Findings, coreFiles(core, result.WordPress)...)

	if res, err = r.exec(ctx, wpcli.PluginVerifyChecksums(), r.job.Site.Path); err != nil {
		return err
	}
	plugins, err := wpcli.ParsePluginChecksums(res)
	if err != nil {
		result.Findings = append(result.Findings, notChecked(checkPluginFiles, err))
	}
	result.Findings = append(result.Findings, pluginFiles(plugins.Problems, versions)...)
	for _, slug := range plugins.Skipped {
		result.Findings = append(result.Findings, model.Finding{Kind: model.KindPluginUnverified, Subject: "plugin:" + slug,
			Version: versions[slug], Level: model.LevelInfo})
	}
	return r.uploads(ctx, result)
}

// coreFiles makes one finding of the WordPress files that differ, how many changed, went missing
// or were added, and the first ones; each of the files often deleted on purpose has its own
// finding, only shown.
func coreFiles(problems []wpcli.FileProblem, wordpress string) []model.Finding {
	var findings []model.Finding
	counts := map[wpcli.FileChange]int{}
	var files []string
	for _, p := range problems {
		if p.Change == wpcli.FileMissing && slices.Contains(removable, p.File) {
			findings = append(findings, model.Finding{Kind: model.KindCoreFile, Subject: "core", Identifier: p.File,
				Version: wordpress, Level: model.LevelInfo, Facts: map[string]string{"change": string(p.Change)}})
			continue
		}
		counts[p.Change]++
		if len(files) < filesShown {
			files = append(files, p.File)
		}
	}
	if len(files) == 0 {
		return findings
	}
	return append(findings, model.Finding{Kind: model.KindCoreFile, Subject: "core", Version: wordpress, Level: model.LevelUrgent,
		Facts: map[string]string{
			"changed": strconv.Itoa(counts[wpcli.FileChanged]), "missing": strconv.Itoa(counts[wpcli.FileMissing]),
			"added": strconv.Itoa(counts[wpcli.FileAdded]), "files": strings.Join(files, ", "),
		}})
}

// pluginFiles makes one finding per plugin of its files that differ: how many were added and
// changed, and the first ones. Many added and none changed means wordpress.org's list does not
// match this version's files at all: they could not be compared, which is only shown. Many added
// and few changed means another plugin of the same name as wordpress.org's.
func pluginFiles(problems []wpcli.FileProblem, versions map[string]string) []model.Finding {
	var order []string
	byPlugin := map[string][]wpcli.FileProblem{}
	for _, p := range problems {
		if _, ok := byPlugin[p.Plugin]; !ok {
			order = append(order, p.Plugin)
		}
		byPlugin[p.Plugin] = append(byPlugin[p.Plugin], p)
	}
	var findings []model.Finding
	for _, plugin := range order {
		var added, changed int
		var files []string
		for _, p := range byPlugin[plugin] {
			if p.Change == wpcli.FileAdded {
				added++
			} else {
				changed++
			}
			if len(files) < filesShown {
				files = append(files, p.File)
			}
		}
		facts := map[string]string{"added": strconv.Itoa(added), "changed": strconv.Itoa(changed), "files": strings.Join(files, ", ")}
		level := model.LevelAttention
		switch {
		case added >= mismatchAdded && changed == 0:
			facts["uncomparable"], level = "true", model.LevelInfo
		case added >= mismatchAdded && added > mismatchRatio*changed:
			facts["mismatch"] = "true"
		}
		findings = append(findings, model.Finding{Kind: model.KindPluginFile, Subject: "plugin:" + plugin,
			Version: versions[plugin], Level: level, Facts: facts})
	}
	return findings
}

// realRoot reads the site's folder with its links resolved, once: the uploads and debug.log are
// found from it.
func (r *run) realRoot(ctx context.Context) (string, error) {
	if r.root != "" {
		return r.root, nil
	}
	out, err := r.shell(ctx, func() (string, error) { return remote.RealRoot(r.job.Site.Path) })
	if err != nil {
		return "", err
	}
	if r.root, err = wpcli.ParseDir(out); err != nil {
		return "", err
	}
	return r.root, nil
}

// uploads lists PHP files under the uploads folder of the site's real folder.
func (r *run) uploads(ctx context.Context, result *model.Scan) error {
	dir, err := r.realRoot(ctx)
	var out []byte
	if err == nil {
		out, err = r.shell(ctx, func() (string, error) { return remote.UploadsPHP(dir) })
	}
	if err != nil {
		if stops(ctx, err) {
			return err
		}
		result.Findings = append(result.Findings, notChecked(checkUploads, err))
		return nil
	}
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

// settings reads wp-config.php's constants, the options, debug.log and WP-Cron. Whether the site
// answers in HTTPS is asked from the machine: WP-CLI reads the address without the plugins and
// wp-config.php switches that make it https on a request.
func (r *run) settings(ctx context.Context, result *model.Scan) error {
	values, err := r.constants(ctx, result)
	if err != nil {
		return err
	}
	if err := r.adminEmail(ctx, result); err != nil {
		return err
	}
	if values != nil {
		if err := r.debugLog(ctx, result, values); err != nil {
			return err
		}
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
	if err := r.cron(ctx, result); err != nil {
		return err
	}
	return r.autoload(ctx, result)
}

// The constants of wp-config.php the rules read; only those defined are asked.
var constants = []string{"WP_DEBUG", "WP_DEBUG_DISPLAY", "WP_DEBUG_LOG", "DISALLOW_FILE_EDIT"}

// constants reads whether files can be edited from the administration, and whether PHP errors
// show on the pages (WP_DEBUG, with WP_DEBUG_DISPLAY true by default). It returns the values read,
// a constant not defined left out; nil when they could not be read.
func (r *run) constants(ctx context.Context, result *model.Scan) (map[string]any, error) {
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
			return nil, err
		}
		result.Findings = append(result.Findings, notChecked(checkSettings, err))
		return nil, nil
	}
	if v, ok := values["DISALLOW_FILE_EDIT"]; !ok || !phpTrue(v) {
		result.Findings = append(result.Findings, model.Finding{Kind: model.KindFileEditing, Subject: "site", Level: model.LevelAttention})
	}
	if display, ok := values["WP_DEBUG_DISPLAY"]; phpTrue(values["WP_DEBUG"]) && (!ok || phpTrue(display)) {
		result.Findings = append(result.Findings, model.Finding{Kind: model.KindErrorsDisplayed, Subject: "site", Level: model.LevelAttention})
	}
	return values, nil
}

// adminEmail reads where WordPress sends its notices, and the address waiting for its
// confirmation (ADR-0028).
func (r *run) adminEmail(ctx context.Context, result *model.Scan) error {
	for name, into := range map[string]*string{"admin_email": &result.AdminEmail, "new_admin_email": &result.AdminEmailPending} {
		res, err := r.exec(ctx, wpcli.OptionGet(name), r.job.Site.Path)
		if err == nil {
			*into, _, err = wpcli.ParseOption(res)
		}
		if err != nil {
			if stops(ctx, err) {
				return err
			}
			result.Findings = append(result.Findings, notChecked(checkSettings, err))
			return nil
		}
	}
	if result.AdminEmailPending == result.AdminEmail {
		result.AdminEmailPending = "" // what WordPress leaves once confirmed
	}
	return nil
}

// debugLog reads debug.log where WordPress writes it, or where one left from before would be: its
// size, and its fatal errors of the last week as one finding (ADR-0028).
func (r *run) debugLog(ctx context.Context, result *model.Scan, values map[string]any) error {
	log, err := wpcli.DebugLogOf(values["WP_DEBUG"], values["WP_DEBUG_LOG"])
	var root, file string
	if err == nil {
		root, err = r.realRoot(ctx)
	}
	if err == nil {
		file, err = wpcli.DebugLogFile(root, log)
	}
	var read remote.DebugLogRead
	if err == nil {
		var out []byte
		if out, err = r.shell(ctx, func() (string, error) { return remote.DebugLog(file) }); err == nil {
			read, err = remote.ParseDebugLog(out, r.scanner.Now())
		}
	}
	if err != nil {
		if stops(ctx, err) {
			return err
		}
		result.Findings = append(result.Findings, notChecked(checkDebugLogFile, err))
		return nil
	}
	if !read.Exists && !log.Enabled {
		return nil
	}
	result.DebugLog = model.DebugLog{Path: strings.TrimPrefix(file, root+"/"), Size: read.Size, Written: log.Enabled}
	if read.Recent > 0 {
		result.Findings = append(result.Findings, model.Finding{Kind: model.KindDebugFatal, Subject: "site", Level: model.LevelAttention,
			Facts: map[string]string{"count": strconv.Itoa(read.Recent), "last": read.Last.Format(time.RFC3339), "message": read.Message}})
	}
	return nil
}

// phpWeb reads which PHP version serves the pages, from the dashboard's checks WordPress keeps a
// week (ADR-0028).
func (r *run) phpWeb(ctx context.Context, result *model.Scan) error {
	res, err := r.exec(ctx, wpcli.PHPChecks(), r.job.Site.Path)
	if err == nil {
		var checks []wpcli.PHPCheck
		var cached bool
		if checks, cached, err = wpcli.ParsePHPChecks(res); err == nil {
			result.PHPWeb = wpcli.WebPHP(checks, cached, result.PHPCLI, r.scanner.Now())
			return nil
		}
	}
	if stops(ctx, err) {
		return err
	}
	result.Findings = append(result.Findings, notChecked(checkPHPWeb, err))
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
