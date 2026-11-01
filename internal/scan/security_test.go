package scan_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/lookup"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// lookups answers the outside lookups of a scan from its fields; a missing answer is an empty one.
type lookups struct {
	mu      sync.Mutex
	asked   []string
	plugins map[string]lookup.Report
	core    lookup.Report
	php     lookup.PHPReport
	info    map[string]lookup.PluginInfo
	cert    lookup.Certificate
	domain  lookup.DomainExpiry
	debug   lookup.DebugLogCheck
	err     error // answered by every lookup when set
	certErr error // answered by the certificate's lookup when set
}

func (l *lookups) ask(what string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.asked = append(l.asked, what)
	return l.err
}

func (l *lookups) Core(_ context.Context, wordpress string) (lookup.Report, error) {
	return l.core, l.ask("core/" + wordpress)
}

func (l *lookups) Plugin(_ context.Context, slug string) (lookup.Report, error) {
	return l.plugins[slug], l.ask("plugin/" + slug)
}

func (l *lookups) Theme(_ context.Context, slug string) (lookup.Report, error) {
	return lookup.Report{}, l.ask("theme/" + slug)
}

func (l *lookups) PHP(_ context.Context, php string) (lookup.PHPReport, error) {
	return l.php, l.ask("php/" + php)
}

func (l *lookups) PluginInfo(_ context.Context, slug string) (lookup.PluginInfo, error) {
	return l.info[slug], l.ask("info/" + slug)
}

func (l *lookups) Certificate(_ context.Context, host string) (lookup.Certificate, error) {
	if err := l.ask("certificate/" + host); err != nil {
		return lookup.Certificate{}, err
	}
	return l.cert, l.certErr
}

func (l *lookups) DomainExpiry(_ context.Context, host string) (lookup.DomainExpiry, error) {
	return l.domain, l.ask("domain/" + host)
}

func (l *lookups) DebugLog(_ context.Context, home string) (lookup.DebugLogCheck, error) {
	return l.debug, l.ask("debug-log/" + home)
}

// The scans' clock starts on 2026-10-29 at 09:00 UTC.
var day0 = time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)

// find returns the finding of a kind and subject (and identifier, when given).
func find(t *testing.T, s model.Scan, kind model.FindingKind, subject, identifier string) model.Finding {
	t.Helper()
	for _, f := range s.Findings {
		if f.Kind == kind && f.Subject == subject && (identifier == "" || f.Identifier == identifier) {
			return f
		}
	}
	t.Fatalf("no finding %s %s %s in %+v", kind, subject, identifier, s.Findings)
	return model.Finding{}
}

func has(s model.Scan, kind model.FindingKind) bool {
	return slices.ContainsFunc(s.Findings, func(f model.Finding) bool { return f.Kind == kind })
}

func TestSecurityScanOfTheSecuritySite(t *testing.T) {
	l := &lookups{
		plugins: map[string]lookup.Report{"akismet": {Known: true, Vulnerabilities: []lookup.Vulnerability{
			{UUID: "fixed-before", Name: "Akismet < 5.0", MaxVersion: "5.0", MaxOperator: "lt", Severity: "critical"},
			{UUID: "affects", Name: "Akismet < 6.0", MaxVersion: "6.0", MaxOperator: "lt", Severity: "high", Score: "8.1",
				Sources: []lookup.Source{{ID: "CVE-2026-1", Link: "https://www.cve.org/CVERecord?id=CVE-2026-1"}}},
		}}},
		core: lookup.Report{Vulnerabilities: []lookup.Vulnerability{{UUID: "core-1", Name: "WordPress 7.x"}}},
		php:  lookup.PHPReport{Report: lookup.Report{Known: true}, Status: "s", EndOfLife: "2028-12-31"},
		info: map[string]lookup.PluginInfo{
			"wordfence": {OnWordPressOrg: true, Closed: true, ClosedDate: "2026-09-01", ClosedReason: "security-issue"},
			"hello":     {OnWordPressOrg: true, LastUpdated: day0.AddDate(-3, 0, 0)},
		},
		cert:   lookup.Certificate{Host: "security.example.test", NotAfter: day0.Add(5 * 24 * time.Hour), Issuer: "Bench CA"},
		domain: lookup.DomainExpiry{Domain: "example.test", Reason: lookup.NotPublished},
		debug:  lookup.DebugLogCheck{URL: "https://security.example.test/wp-content/debug.log", Status: 206, Public: true},
	}
	s := scanner(replay(t, "security"))
	s.Lookups = l
	got, _ := s.Site(context.Background(), job("security"), nil)
	if got.Outcome != model.ScanCompleted {
		t.Fatalf("outcome = %q (%s)", got.Outcome, got.Error)
	}
	if got.FilesChecked.IsZero() {
		t.Error("files not compared on a first scan")
	}

	for _, tc := range []struct {
		kind              model.FindingKind
		subject, id, fact string
		level             model.FindingLevel
	}{
		// On the server.
		// One finding for WordPress's files: functions.php changed, freedoms.php missing, fixture-added.php
		// and the container image's wp-config-docker.php added.
		{model.KindCoreFile, "core", "", "added=2", model.LevelUrgent},
		{model.KindCoreFile, "core", "", "missing=1", model.LevelUrgent},
		{model.KindPluginFile, "plugin:akismet", "", "added=1", model.LevelAttention},
		{model.KindPluginUnverified, "plugin:fixture-premium", "", "", model.LevelInfo},
		{model.KindUploadsPHP, "site", "2026/10/fixture-shell.php", "", model.LevelUrgent},
		{model.KindUploadsPHP, "site", "fixture-old.PHTML", "", model.LevelUrgent},
		{model.KindAdministrator, "user:admin", "", "two-factor=off", model.LevelAttention},
		{model.KindAdministrator, "user:marie", "", "two-factor=off", model.LevelAttention},
		{model.KindAdministrator, "user:totp-admin", "", "two-factor=on", model.LevelInfo},
		{model.KindAdministrator, "user:wp2fa-admin", "", "two-factor=on", model.LevelInfo},
		{model.KindAdministrator, "user:wf-admin", "", "two-factor=on", model.LevelInfo},
		{model.KindFileEditing, "site", "", "", model.LevelAttention},
		{model.KindErrorsDisplayed, "site", "", "", model.LevelAttention},
		{model.KindHiddenFromSearch, "site", "", "", model.LevelAttention},
		{model.KindCronOverdue, "site", "", "hook=fixture_overdue", model.LevelAttention},
		{model.KindAutoloadSize, "site", "", "limit=800000", model.LevelAttention},
		{model.KindDatabaseVersion, "site", "", "version=11.8.9-MariaDB-ubu2404", model.LevelInfo},
		// The fatal error of the day the bench was made; the one of January is too old.
		{model.KindDebugFatal, "site", "", "count=1", model.LevelAttention},
		// From the machine.
		{model.KindVulnerability, "plugin:akismet", "affects", "severity=high", model.LevelUrgent},
		{model.KindVulnerability, "core", "core-1", "name=WordPress 7.x", model.LevelAttention},
		{model.KindPHPSupport, "php", "", "on=web", model.LevelAttention},
		{model.KindPluginClosed, "plugin:wordfence", "", "reason=security-issue", model.LevelUrgent},
		{model.KindPluginAbandoned, "plugin:hello", "", "last-release=2023-10-29", model.LevelAttention},
		{model.KindCertificate, "site", "", "issuer=Bench CA", model.LevelUrgent},
		{model.KindDomain, "site", "", "reason=" + lookup.NotPublished, model.LevelInfo},
		{model.KindDebugLog, "site", "", "url=https://security.example.test/wp-content/debug.log", model.LevelUrgent},
	} {
		f := find(t, got, tc.kind, tc.subject, tc.id)
		if f.Level != tc.level {
			t.Errorf("%s %s %s: level %q, want %q", tc.kind, tc.subject, tc.id, f.Level, tc.level)
		}
		if key, value, ok := strings.Cut(tc.fact, "="); ok && f.Facts[key] != value {
			t.Errorf("%s %s %s: %s = %q, want %q", tc.kind, tc.subject, tc.id, key, f.Facts[key], value)
		}
	}

	// A vulnerability fixed before the installed version is not one; the others carry the installed
	// version, so that ignoring them until the next one works.
	for _, f := range got.Findings {
		if f.Identifier == "fixed-before" {
			t.Errorf("a record fixed in 5.0 reported for 5.7.2: %+v", f)
		}
	}
	if f := find(t, got, model.KindVulnerability, "plugin:akismet", "affects"); f.Version != "5.7.2" || f.Facts["fixed"] != "6.0" ||
		f.Facts["sources"] != "https://www.cve.org/CVERecord?id=CVE-2026-1" {
		t.Errorf("akismet's vulnerability = %+v", f)
	}
	if f := find(t, got, model.KindCertificate, "site", ""); f.Version != "2026-11-03" {
		t.Errorf("certificate's version = %q, want its expiry date", f.Version)
	}
	if has(got, model.KindCheck) {
		t.Errorf("checks not run: %+v", got.Findings)
	}
	// The complements (ADR-0028): the dashboard's check, made by WordPress on the bench, names the
	// PHP serving the pages (the command line's there); a debug.log left from before, logging off.
	if got.PHPWeb.Version != got.PHPCLI || got.PHPWeb.Seen.IsZero() || got.PHPWeb.Unknown != model.PHPWebRead {
		t.Errorf("PHP on the web = %+v, want %s with its date", got.PHPWeb, got.PHPCLI)
	}
	if got.AdminEmail != "admin@example.test" || got.AdminEmailPending != "webmaster@example.test" {
		t.Errorf("admin email = %q, pending %q", got.AdminEmail, got.AdminEmailPending)
	}
	if d := got.DebugLog; d.Path != "wp-content/debug.log" || d.Written || d.Size == 0 {
		t.Errorf("debug.log = %+v, want one left from before in wp-content", d)
	}
	if f := find(t, got, model.KindDebugFatal, "site", ""); !strings.Contains(f.Facts["message"], "fixture_missing()") || f.Facts["last"] == "" {
		t.Errorf("fatal errors = %+v", f.Facts)
	}
	// Worst first.
	if got.Findings[0].Level != model.LevelUrgent || got.Findings[len(got.Findings)-1].Level != model.LevelInfo {
		t.Errorf("findings not ordered: first %+v, last %+v", got.Findings[0], got.Findings[len(got.Findings)-1])
	}
	// Plugins and themes asked about by slug; must-use plugins and drop-ins never.
	for _, want := range []string{"plugin/akismet", "info/fixture-premium", "theme/twentytwentyfive", "php/8.4.27", "core/" + got.WordPress,
		"certificate/security.example.test", "domain/security.example.test", "debug-log/https://security.example.test"} {
		if !slices.Contains(l.asked, want) {
			t.Errorf("%s not asked; asked %v", want, l.asked)
		}
	}
}

// A site whose dashboard noted nothing and that keeps no debug.log: said so, and the PHP checks are
// about the command line's version.
func TestComplementsOfAPlainSite(t *testing.T) {
	l := &lookups{php: lookup.PHPReport{Report: lookup.Report{Known: true}, Status: "s"}}
	s := scanner(replay(t, "plain"))
	s.Lookups = l
	got, _ := s.Site(context.Background(), job("plain"), nil)
	if got.PHPWeb != (model.PHPWeb{Unknown: model.PHPWebNotNoted}) {
		t.Errorf("PHP on the web = %+v, want not noted", got.PHPWeb)
	}
	if got.DebugLog != (model.DebugLog{}) || has(got, model.KindDebugFatal) {
		t.Errorf("debug.log = %+v", got.DebugLog)
	}
	if got.AdminEmail != "admin@example.test" || got.AdminEmailPending != "" {
		t.Errorf("admin email = %q, pending %q", got.AdminEmail, got.AdminEmailPending)
	}
	if f := find(t, got, model.KindPHPSupport, "php", ""); f.Facts["on"] != "cli" || f.Version != got.PHPCLI {
		t.Errorf("PHP support = %+v, want the command line's", f)
	}
}

// Without the lookups, or with a destination turned off, their checks say so: never "all good".
func TestLookupsNotMadeAreNotChecked(t *testing.T) {
	got, _ := scanner(replay(t, "plain")).Site(context.Background(), job("plain"), nil)
	for _, check := range []string{"vulnerabilities", "wordpress.org", "certificate", "domain", "debug-log"} {
		if f := find(t, got, model.KindCheck, "site", check); f.Level != model.LevelNotChecked || f.Facts["reason"] != "off" {
			t.Errorf("check %s = %+v, want not checked, turned off", check, f)
		}
	}

	s := scanner(replay(t, "plain"))
	s.Lookups = &lookups{err: errors.New("no network")}
	got, _ = s.Site(context.Background(), job("plain"), nil)
	if f := find(t, got, model.KindCheck, "site", "vulnerabilities"); f.Facts["reason"] != "error" || f.Facts["error"] != "no network" {
		t.Errorf("check vulnerabilities = %+v, want the error", f)
	}
}

// The rules that rank what comes from the machine, one case each.
func TestLookupRules(t *testing.T) {
	vuln := func(v lookup.Vulnerability) *lookups {
		v.UUID, v.MaxVersion, v.MaxOperator = "v", "9.0", "lt"
		return &lookups{plugins: map[string]lookup.Report{"akismet": {Known: true, Vulnerabilities: []lookup.Vulnerability{v}}}}
	}
	for _, tc := range []struct {
		name    string
		lookups *lookups
		kind    model.FindingKind
		subject string
		want    model.FindingLevel
	}{
		{"critical", vuln(lookup.Vulnerability{Severity: "critical"}), model.KindVulnerability, "plugin:akismet", model.LevelUrgent},
		{"high", vuln(lookup.Vulnerability{Severity: "high"}), model.KindVulnerability, "plugin:akismet", model.LevelUrgent},
		{"medium", vuln(lookup.Vulnerability{Severity: "medium"}), model.KindVulnerability, "plugin:akismet", model.LevelAttention},
		{"not scored", vuln(lookup.Vulnerability{}), model.KindVulnerability, "plugin:akismet", model.LevelAttention},
		{"known exploited", vuln(lookup.Vulnerability{Severity: "low", KEV: true}), model.KindVulnerability, "plugin:akismet", model.LevelUrgent},
		{"not fixed", vuln(lookup.Vulnerability{Severity: "low", Unfixed: true}), model.KindVulnerability, "plugin:akismet", model.LevelUrgent},
		{"closed for security", &lookups{info: map[string]lookup.PluginInfo{"akismet": {OnWordPressOrg: true, Closed: true, ClosedReason: "security-issue"}}},
			model.KindPluginClosed, "plugin:akismet", model.LevelUrgent},
		{"closed by its author", &lookups{info: map[string]lookup.PluginInfo{"akismet": {OnWordPressOrg: true, Closed: true, ClosedReason: "author-request"}}},
			model.KindPluginClosed, "plugin:akismet", model.LevelAttention},
		{"PHP with security fixes only", &lookups{php: lookup.PHPReport{Status: "s"}}, model.KindPHPSupport, "php", model.LevelAttention},
		{"PHP unmaintained", &lookups{php: lookup.PHPReport{Status: "d"}}, model.KindPHPSupport, "php", model.LevelAttention},
		{"certificate for another name", &lookups{cert: lookup.Certificate{NotAfter: day0.AddDate(1, 0, 0), Problem: lookup.WrongName}},
			model.KindCertificate, "site", model.LevelUrgent},
		{"certificate in 7 days", &lookups{cert: lookup.Certificate{NotAfter: day0.Add(7 * 24 * time.Hour)}}, model.KindCertificate, "site", model.LevelUrgent},
		{"certificate in 20 days", &lookups{cert: lookup.Certificate{NotAfter: day0.Add(20 * 24 * time.Hour)}}, model.KindCertificate, "site", model.LevelAttention},
		{"certificate in 60 days", &lookups{cert: lookup.Certificate{NotAfter: day0.Add(60 * 24 * time.Hour)}}, model.KindCertificate, "site", model.LevelInfo},
		{"domain in 3 days", &lookups{domain: lookup.DomainExpiry{Domain: "example.com", Expires: day0.Add(3 * 24 * time.Hour)}},
			model.KindDomain, "site", model.LevelUrgent},
		{"domain in 30 days", &lookups{domain: lookup.DomainExpiry{Domain: "example.com", Expires: day0.Add(29 * 24 * time.Hour)}},
			model.KindDomain, "site", model.LevelAttention},
		{"domain in a year", &lookups{domain: lookup.DomainExpiry{Domain: "example.com", Expires: day0.AddDate(1, 0, 0)}},
			model.KindDomain, "site", model.LevelInfo},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := scanner(replay(t, "plain"))
			s.Lookups = tc.lookups
			got, _ := s.Site(context.Background(), job("plain"), nil)
			if f := find(t, got, tc.kind, tc.subject, ""); f.Level != tc.want {
				t.Errorf("level %q, want %q (%+v)", f.Level, tc.want, f)
			}
		})
	}
}

func checksums(calls []string) int {
	n := 0
	for _, c := range calls {
		if strings.Contains(c, "'verify-checksums'") {
			n++
		}
	}
	return n
}

// The files are compared at most once a day: in between, the last comparison is carried over,
// unless asked, or WordPress or a plugin changed version.
func TestFilesAreComparedAtMostOnceADay(t *testing.T) {
	ctx := context.Background()
	j := job("security")
	first, _ := scanner(replay(t, "security")).Site(ctx, j, nil)
	if first.Outcome != model.ScanCompleted || !has(first, model.KindCoreFile) {
		t.Fatalf("first scan = %q, %+v", first.Outcome, first.Findings)
	}

	at := func(later time.Duration, check bool, previous model.Scan) (model.Scan, int) {
		fake := replay(t, "security")
		s := scanner(fake)
		s.Now = ticking(day0.Add(later))
		next := j
		next.Previous, next.CheckFiles = previous, check
		got, _ := s.Site(ctx, next, nil)
		return got, checksums(fake.Calls())
	}

	got, n := at(time.Hour, false, first)
	if n != 0 || !got.FilesChecked.Equal(first.FilesChecked) {
		t.Errorf("an hour later: %d checksum commands, files checked %v; want none, %v", n, got.FilesChecked, first.FilesChecked)
	}
	if f := find(t, got, model.KindCoreFile, "core", ""); f.Level != model.LevelUrgent || f.Facts["missing"] != "1" {
		t.Errorf("carried finding = %+v", f)
	}
	if _, n := at(time.Hour, true, first); n != 2 {
		t.Errorf("asked: %d checksum commands, want 2", n)
	}
	if _, n := at(25*time.Hour, false, first); n != 2 {
		t.Errorf("a day later: %d checksum commands, want 2", n)
	}
	updated := first
	updated.Items = slices.Clone(first.Items)
	for i, it := range updated.Items {
		if it.Kind == model.ItemPlugin && it.Slug == "akismet" {
			updated.Items[i].Version = "5.6"
		}
	}
	if _, n := at(time.Hour, false, updated); n != 2 {
		t.Errorf("after a plugin's update: %d checksum commands, want 2", n)
	}
	failed := first
	failed.Findings = append(slices.Clone(first.Findings), model.Finding{Kind: model.KindCheck, Subject: "site", Identifier: "core-files",
		Level: model.LevelNotChecked})
	if _, n := at(time.Hour, false, failed); n != 2 {
		t.Errorf("after a failed comparison: %d checksum commands, want 2", n)
	}
}

// A security read that fails on its own leaves its check "not checked"; the scan completes.
func TestAFailedSecurityReadIsNotChecked(t *testing.T) {
	fake := replay(t, "plain")
	line, err := wpcli.CoreVerifyChecksums().Remote("wp", "~/sites/plain")
	if err != nil {
		t.Fatal(err)
	}
	// A server that cannot reach wordpress.org for the checksums.
	fake.Set(line, sshx.Result{ExitCode: 1, Stderr: []byte("Error: Couldn't get checksums from WordPress.org.\n")})
	got, _ := scanner(fake).Site(context.Background(), job("plain"), nil)
	if got.Outcome != model.ScanCompleted {
		t.Fatalf("outcome = %q (%s)", got.Outcome, got.Error)
	}
	if f := find(t, got, model.KindCheck, "site", "core-files"); f.Level != model.LevelNotChecked || !strings.Contains(f.Facts["error"], "Couldn't get checksums") {
		t.Errorf("core files = %+v, want not checked with the reason", f)
	}
}

// Whether wordpress.org knows a plugin comes from wordpress.org itself, asked from the machine:
// the server's own reading says "closed" when it cannot reach it (Hello Dolly here). Without an
// answer, the server's reading stays.
func TestWordPressOrgAnswersForItsPlugins(t *testing.T) {
	onWordPressOrg := func(s model.Scan, slug string) bool {
		got, ok := item(s, model.ItemPlugin, slug)
		if !ok {
			t.Fatalf("no plugin %s", slug)
		}
		return got.OnWordPressOrg
	}
	server, _ := scanner(replay(t, "outdated")).Site(context.Background(), job("outdated"), nil)
	if onWordPressOrg(server, "hello") || !onWordPressOrg(server, "akismet") {
		t.Fatal("without lookups, the server's reading is not kept")
	}

	s := scanner(replay(t, "outdated"))
	s.Lookups = &lookups{info: map[string]lookup.PluginInfo{
		"akismet": {OnWordPressOrg: true},
		"hello":   {OnWordPressOrg: true},
		// fixture-premium: wordpress.org does not know it.
	}}
	got, _ := s.Site(context.Background(), job("outdated"), nil)
	if !onWordPressOrg(got, "hello") || !onWordPressOrg(got, "akismet") || onWordPressOrg(got, "fixture-premium") {
		t.Errorf("hello %v, akismet %v, fixture-premium %v; want wordpress.org's answers",
			onWordPressOrg(got, "hello"), onWordPressOrg(got, "akismet"), onWordPressOrg(got, "fixture-premium"))
	}
	if p, _ := item(got, model.ItemPlugin, "fixture-premium"); !p.UpdateSourceUnknown() {
		t.Errorf("fixture-premium = %+v, want its update source unknown", p)
	}
}

// What a real host showed: WordPress files deleted on purpose, and a plugin of the user's own
// whose name wordpress.org also has, its files compared with another plugin's.
func TestFilesDeletedOnPurposeAndAPluginOfTheSameName(t *testing.T) {
	fake := replay(t, "plain")
	core, err := wpcli.CoreVerifyChecksums().Remote("wp", "~/sites/plain")
	if err != nil {
		t.Fatal(err)
	}
	fake.Set(core, sshx.Result{ExitCode: 1, Stderr: []byte("Warning: File doesn't exist: wp-config-sample.php\n" +
		"Warning: File doesn't exist: readme.html\nWarning: File doesn't exist: wp-login.php\n" +
		"Error: WordPress installation doesn't verify against checksums.\n")})
	plugins, err := wpcli.PluginVerifyChecksums().Remote("wp", "~/sites/plain")
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for i := range 12 {
		files = append(files, fmt.Sprintf(`{"plugin_name":"akismet","file":"app/f%d.php","message":"File was added"}`, i))
	}
	files = append(files, `{"plugin_name":"akismet","file":"akismet.php","message":"Checksum does not match"}`)
	// wordpress.org's list matches none of hello's files: they cannot be compared.
	for i := range 10 {
		files = append(files, fmt.Sprintf(`{"plugin_name":"hello","file":"h%d.php","message":"File was added"}`, i))
	}
	fake.Set(plugins, sshx.Result{ExitCode: 1, Stdout: []byte("[" + strings.Join(files, ",") + "]"),
		Stderr: []byte("Error: No plugins verified (2 failed).\n")})

	s := scanner(fake)
	s.Lookups = &lookups{info: map[string]lookup.PluginInfo{
		"akismet": {OnWordPressOrg: true, Closed: true, ClosedReason: "security-issue"},
		"hello":   {OnWordPressOrg: true},
	}}
	got, _ := s.Site(context.Background(), job("plain"), nil)
	for _, file := range []string{"wp-config-sample.php", "readme.html"} {
		if f := find(t, got, model.KindCoreFile, "core", file); f.Level != model.LevelInfo {
			t.Errorf("%s missing: %q, want only shown", file, f.Level)
		}
	}
	if f := find(t, got, model.KindCoreFile, "core", ""); f.Level != model.LevelUrgent || f.Facts["missing"] != "1" || f.Facts["files"] != "wp-login.php" {
		t.Errorf("WordPress's files = %+v, want wp-login.php missing, urgent", f)
	}
	plugin := find(t, got, model.KindPluginFile, "plugin:akismet", "")
	if plugin.Facts["added"] != "12" || plugin.Facts["changed"] != "1" || plugin.Facts["mismatch"] != "true" ||
		strings.Count(plugin.Facts["files"], ",") != 9 {
		t.Errorf("akismet's files = %+v, want one finding, 12 added, 1 changed, a mismatch, 10 files named", plugin.Facts)
	}
	// The closed akismet of wordpress.org is not the one installed: the closure is only shown.
	if f := find(t, got, model.KindPluginClosed, "plugin:akismet", ""); f.Level != model.LevelInfo || f.Facts["other-plugin"] != "true" {
		t.Errorf("closure of another plugin of the same name = %+v, want only shown", f)
	}
	if f := find(t, got, model.KindPluginFile, "plugin:hello", ""); f.Level != model.LevelInfo || f.Facts["uncomparable"] != "true" || f.Facts["mismatch"] != "" {
		t.Errorf("hello's files = %+v, want not comparable, only shown", f)
	}
}

// WordPress can store http:// and still be served in HTTPS (coolectif.ch): the machine asks the
// site itself. Only an http:// address with no TLS answer is reported.
func TestHTTPSIsAskedOfTheSiteItself(t *testing.T) {
	scanHTTP := func(l *lookups) model.Scan {
		fake := replay(t, "plain")
		home, err := wpcli.OptionGet("home").Remote("wp", "~/sites/plain")
		if err != nil {
			t.Fatal(err)
		}
		fake.Set(home, sshx.Result{Stdout: []byte("http://plain.example.test\n")})
		s := scanner(fake)
		s.Lookups = l
		got, _ := s.Site(context.Background(), job("plain"), nil)
		return got
	}

	served := &lookups{cert: lookup.Certificate{NotAfter: day0.AddDate(0, 3, 0), Issuer: "Bench CA"}}
	got := scanHTTP(served)
	if has(got, model.KindNoHTTPS) || find(t, got, model.KindCertificate, "site", "").Level != model.LevelInfo {
		t.Errorf("served in HTTPS: %+v", got.Findings)
	}
	if !slices.Contains(served.asked, "debug-log/https://plain.example.test") {
		t.Errorf("debug.log asked as %v, want in HTTPS", served.asked)
	}

	got = scanHTTP(&lookups{certErr: errors.New("connection refused")})
	if f := find(t, got, model.KindNoHTTPS, "site", ""); f.Level != model.LevelAttention || f.Facts["error"] != "connection refused" {
		t.Errorf("no TLS answer: %+v", f)
	}
}
