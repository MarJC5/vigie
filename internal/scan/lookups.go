package scan

import (
	"context"
	"errors"
	"maps"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/MarJC5/vigie/internal/lookup"
	"github.com/MarJC5/vigie/internal/model"
)

// Lookups asks outside services, from the user's machine, what the server cannot tell
// (lookup.Client). A destination the user turned off, or did not agree to, answers lookup.ErrOff.
type Lookups interface {
	Core(ctx context.Context, wordpress string) (lookup.Report, error)
	Plugin(ctx context.Context, slug string) (lookup.Report, error)
	Theme(ctx context.Context, slug string) (lookup.Report, error)
	PHP(ctx context.Context, php string) (lookup.PHPReport, error)
	PluginInfo(ctx context.Context, slug string) (lookup.PluginInfo, error)
	Certificate(ctx context.Context, host string) (lookup.Certificate, error)
	DomainExpiry(ctx context.Context, host string) (lookup.DomainExpiry, error)
	DebugLog(ctx context.Context, home string) (lookup.DebugLogCheck, error)
}

// Outside lookups at a time, across the sites scanned.
const (
	lookupsKey     = "lookups"
	lookupsAtOnce  = 4
	freshAnswerAge = 24 * time.Hour // an answer older than this came from the cache, offline
)

// lookupRun gathers the findings of a site's lookups, made in parallel.
type lookupRun struct {
	scanner  Scanner
	mu       sync.Mutex
	findings []model.Finding
	failed   map[string]error // the first failure of each check
}

func (l *lookupRun) add(f ...model.Finding) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.findings = append(l.findings, f...)
}

func (l *lookupRun) fail(check string, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.failed[check]; !ok {
		l.failed[check] = err
	}
}

// lookups asks the outside services about the site: known vulnerabilities of WordPress, its
// plugins and themes and PHP, wordpress.org's view of the plugins, the certificate, the domain's
// expiry and the debug.log. Without Lookups, every such check is "not checked".
func (s Scanner) lookups(ctx context.Context, scan model.Scan) []model.Finding {
	l := &lookupRun{scanner: s, failed: map[string]error{}}
	if s.Lookups == nil {
		for _, check := range []string{checkVulnerabilities, checkWordPressOrg, checkCertificate, checkDomain, checkDebugLog} {
			l.fail(check, lookup.ErrOff)
		}
		return l.result()
	}
	var wg sync.WaitGroup
	do := func(fn func()) {
		wg.Go(func() {
			// Waits for a slot; a cancelled scan stops waiting and its findings are not kept.
			_ = s.Limiter.Do(ctx, lookupsKey, lookupsAtOnce, func() error { fn(); return nil })
		})
	}
	do(func() { l.core(ctx, scan.WordPress) })
	if scan.PHPCLI != "" {
		do(func() { l.php(ctx, scan.PHPCLI) })
	}
	for _, item := range scan.Items {
		switch {
		case item.Kind == model.ItemPlugin && item.Status != "must-use" && item.Status != "dropin":
			do(func() { l.plugin(ctx, item) })
			do(func() { l.pluginInfo(ctx, item) })
		case item.Kind == model.ItemTheme:
			do(func() { l.theme(ctx, item) })
		}
	}
	if u, err := url.Parse(scan.Home); err == nil && u.Hostname() != "" {
		if u.Scheme == "https" {
			do(func() { l.certificate(ctx, u.Hostname()) })
		}
		do(func() { l.domain(ctx, u.Hostname()) })
		do(func() { l.debugLog(ctx, scan.Home) })
	}
	wg.Wait()
	return l.result()
}

// result adds a finding for each check that failed.
func (l *lookupRun) result() []model.Finding {
	for _, check := range slices.Sorted(maps.Keys(l.failed)) {
		l.findings = append(l.findings, notChecked(check, l.failed[check]))
	}
	return l.findings
}

// vulnerabilities turns the records that affect the installed version into findings; core records
// all apply to the version asked.
func (l *lookupRun) vulnerabilities(subject, installed string, report lookup.Report, all bool) {
	for _, v := range report.Vulnerabilities {
		if !all {
			affected, err := v.Affects(installed)
			if err != nil {
				l.fail(checkVulnerabilities, err)
				continue
			}
			if !affected {
				continue
			}
		}
		facts := map[string]string{"name": v.Name}
		set := func(key, value string) {
			if value != "" {
				facts[key] = value
			}
		}
		set("severity", v.Severity)
		set("score", v.Score)
		if v.KEV {
			facts["kev"] = "true"
		}
		if v.Unfixed {
			facts["unfixed"] = "true"
		}
		if v.MaxOperator == "lt" {
			set("fixed", v.MaxVersion)
		}
		var links []string
		for _, src := range v.Sources {
			if src.Link != "" {
				links = append(links, src.Link)
			}
		}
		set("sources", strings.Join(links, " "))
		l.stale(facts, report.Fetched)
		l.add(model.Finding{Kind: model.KindVulnerability, Subject: subject, Identifier: v.UUID, Version: installed,
			Level: vulnerabilityLevel(v), Facts: facts})
	}
}

// stale notes the date of an answer that came from an older cache, the service being unreachable.
func (l *lookupRun) stale(facts map[string]string, fetched time.Time) {
	if !fetched.IsZero() && l.scanner.Now().Sub(fetched) > freshAnswerAge {
		facts["fetched"] = fetched.UTC().Format(time.RFC3339)
	}
}

func (l *lookupRun) core(ctx context.Context, wordpress string) {
	report, err := l.scanner.Lookups.Core(ctx, wordpress)
	if err != nil {
		l.fail(checkVulnerabilities, err)
		return
	}
	l.vulnerabilities("core", wordpress, report, true)
}

func (l *lookupRun) php(ctx context.Context, php string) {
	report, err := l.scanner.Lookups.PHP(ctx, php)
	if err != nil {
		l.fail(checkVulnerabilities, err)
		return
	}
	l.vulnerabilities("php", php, report.Report, false)
	if report.Status == "s" || report.Status == "d" {
		facts := map[string]string{"status": report.Status, "end": report.EndOfLife}
		l.stale(facts, report.Fetched)
		l.add(model.Finding{Kind: model.KindPHPSupport, Subject: "php", Version: php, Level: model.LevelAttention, Facts: facts})
	}
}

// A plugin or theme whose folder name WordPress's directory would never use is not asked about:
// it cannot be there.
func (l *lookupRun) plugin(ctx context.Context, item model.ScanItem) {
	report, err := l.scanner.Lookups.Plugin(ctx, item.Slug)
	if err != nil {
		if !errors.Is(err, lookup.ErrBadInput) {
			l.fail(checkVulnerabilities, err)
		}
		return
	}
	l.vulnerabilities("plugin:"+item.Slug, item.Version, report, false)
}

func (l *lookupRun) theme(ctx context.Context, item model.ScanItem) {
	report, err := l.scanner.Lookups.Theme(ctx, item.Slug)
	if err != nil {
		if !errors.Is(err, lookup.ErrBadInput) {
			l.fail(checkVulnerabilities, err)
		}
		return
	}
	l.vulnerabilities("theme:"+item.Slug, item.Version, report, false)
}

// pluginInfo reports a plugin closed on wordpress.org, or without a release for two years.
func (l *lookupRun) pluginInfo(ctx context.Context, item model.ScanItem) {
	info, err := l.scanner.Lookups.PluginInfo(ctx, item.Slug)
	if err != nil {
		if !errors.Is(err, lookup.ErrBadInput) {
			l.fail(checkWordPressOrg, err)
		}
		return
	}
	subject := "plugin:" + item.Slug
	switch {
	case info.Closed:
		facts := map[string]string{"date": info.ClosedDate, "reason": info.ClosedReason}
		l.stale(facts, info.Fetched)
		l.add(model.Finding{Kind: model.KindPluginClosed, Subject: subject, Version: item.Version,
			Level: closedLevel(info.ClosedReason), Facts: facts})
	case info.Abandoned(l.scanner.Now()):
		facts := map[string]string{"last-release": info.LastUpdated.UTC().Format(time.DateOnly)}
		l.stale(facts, info.Fetched)
		l.add(model.Finding{Kind: model.KindPluginAbandoned, Subject: subject, Version: item.Version,
			Level: model.LevelAttention, Facts: facts})
	}
}

// certificate reads the certificate the site serves; it is always listed, with its expiry.
func (l *lookupRun) certificate(ctx context.Context, host string) {
	cert, err := l.scanner.Lookups.Certificate(ctx, host)
	if err != nil {
		l.fail(checkCertificate, err)
		return
	}
	level := expiryLevel(cert.NotAfter.Sub(l.scanner.Now()))
	facts := map[string]string{"expires": cert.NotAfter.UTC().Format(time.RFC3339), "issuer": cert.Issuer}
	if cert.Problem != "" {
		level, facts["problem"] = model.LevelUrgent, cert.Problem
	}
	l.add(model.Finding{Kind: model.KindCertificate, Subject: "site", Version: cert.NotAfter.UTC().Format(time.DateOnly),
		Level: level, Facts: facts})
}

// domain reads when the domain's registration ends, where the registry publishes it. An address
// without a registered domain (an IP address, a local name) has none to ask about.
func (l *lookupRun) domain(ctx context.Context, host string) {
	d, err := l.scanner.Lookups.DomainExpiry(ctx, host)
	if errors.Is(err, lookup.ErrBadInput) {
		return
	}
	if err != nil {
		l.fail(checkDomain, err)
		return
	}
	if d.Reason != "" {
		l.add(model.Finding{Kind: model.KindDomain, Subject: "site", Level: model.LevelInfo,
			Facts: map[string]string{"domain": d.Domain, "reason": d.Reason}})
		return
	}
	facts := map[string]string{"domain": d.Domain, "expires": d.Expires.UTC().Format(time.RFC3339)}
	l.stale(facts, d.Fetched)
	l.add(model.Finding{Kind: model.KindDomain, Subject: "site", Version: d.Expires.UTC().Format(time.DateOnly),
		Level: expiryLevel(d.Expires.Sub(l.scanner.Now())), Facts: facts})
}

// debugLog asks the site for its debug.log, as anyone could.
func (l *lookupRun) debugLog(ctx context.Context, home string) {
	check, err := l.scanner.Lookups.DebugLog(ctx, home)
	if err != nil {
		l.fail(checkDebugLog, err)
		return
	}
	if check.Public {
		l.add(model.Finding{Kind: model.KindDebugLog, Subject: "site", Level: model.LevelUrgent,
			Facts: map[string]string{"url": check.URL}})
	}
}
