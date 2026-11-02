//go:build integration

package integration_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/scan"
	"github.com/MarJC5/vigie/internal/sshx"
)

var throughput = flag.Bool("throughput", false, "time the commands of a scan, then scans of many sites at a time (slow)")

// env is a number from the environment, or def.
func env(name string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(name)); err == nil {
		return n
	}
	return def
}

// timed is a runner that keeps how long each command took, by what it runs.
type timed struct {
	inner sshx.Runner
	mu    sync.Mutex
	took  map[string][]time.Duration
}

func (r *timed) Run(ctx context.Context, target sshx.Target, remote string) (sshx.Result, error) {
	return r.RunInput(ctx, target, remote, nil)
}

func (r *timed) RunInput(ctx context.Context, target sshx.Target, remote string, input []byte) (sshx.Result, error) {
	start := time.Now()
	res, err := r.inner.RunInput(ctx, target, remote, input)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.took == nil {
		r.took = map[string][]time.Duration{}
	}
	r.took[commandName(remote)] = append(r.took[commandName(remote)], time.Since(start))
	return res, err
}

// commandName is what a remote command runs, without its path and options: "wp plugin list".
func commandName(remote string) string {
	var words []string
	for _, w := range strings.Fields(remote) {
		w = strings.Trim(w, "'\"")
		if strings.HasPrefix(w, "-") || strings.Contains(w, "/") || strings.Contains(w, "=") {
			continue
		}
		if words = append(words, w); len(words) == 3 {
			break
		}
	}
	return strings.Join(words, " ")
}

// TestScanThroughput measures what a scan costs on the bench's account: first each command of a
// scan of a plain site and of a site with 22 plugins, then SITES copies of the plain one (20)
// scanned as the app does (16 sites at a time), with 1, 2 and 4 commands at a time on the account,
// and as if they were on 4 accounts (CASES chooses others). The bench runs on this computer: a
// shared host's own load is not in these numbers.
func TestScanThroughput(t *testing.T) {
	if !*throughput {
		t.Skip("run with -throughput: it copies a site many times and scans them")
	}
	ctx := context.Background()
	a := sshAccess(t)
	known := a.trusted(t)
	scanner := func(r sshx.Runner) scan.Scanner {
		return scan.Scanner{Runner: r, Limiter: sshx.NewLimiter(), Now: timeNow}
	}
	job := func(path, hostKey string, max int) scan.Job {
		return scan.Job{Site: model.Site{ID: model.NewID(), Name: path, Path: path}, Target: a.target(a.key), HostKey: hostKey, Max: max, WPCLI: "wp"}
	}

	// Each command of one scan, the second of three (the first opens the shared connection).
	timeScan := func(label, path string) {
		var one *timed
		var wall []time.Duration
		for i := range 3 {
			r := &timed{inner: a.runner(t, known)}
			start := time.Now()
			s, _ := scanner(r).Site(ctx, job(path, "bench", 2), nil)
			wall = append(wall, time.Since(start))
			if s.Outcome != model.ScanCompleted {
				t.Fatalf("%s: %s %s", path, s.Outcome, s.Error)
			}
			if i == 1 {
				one = r
			}
		}
		type line struct {
			name  string
			count int
			total time.Duration
		}
		var lines []line
		var sum time.Duration
		for name, took := range one.took {
			l := line{name: name, count: len(took)}
			for _, d := range took {
				l.total += d
			}
			sum += l.total
			lines = append(lines, l)
		}
		slices.SortFunc(lines, func(x, y line) int { return int(y.total - x.total) })
		t.Logf("one scan of %s: %s, %s, %s (commands one after the other: %s)", label, wall[0].Round(time.Millisecond),
			wall[1].Round(time.Millisecond), wall[2].Round(time.Millisecond), sum.Round(time.Millisecond))
		for _, l := range lines {
			t.Logf("  %-34s %2d× %6s", l.name, l.count, l.total.Round(time.Millisecond))
		}
	}
	timeScan("a plain site", "~/sites/plain")

	// A site as most are: 20 more plugins from wordpress.org, in French. CASES=none stops after it.
	many := `set -e
P=~/sites/many
rm -rf "$P"; cp -R ~/sites/plain "$P"; rm -rf "$P/node_modules"
wp config set table_prefix many_ --path="$P" --quiet
wp core install --path="$P" --url=https://many.example.test --title=Many --admin_user=admin \
	--admin_password=fixture-only --admin_email=admin@example.test --skip-email --quiet
# Three tries each: a download from wordpress.org fails now and then.
for p in contact-form-7 wordpress-seo classic-editor wpforms-lite really-simple-ssl litespeed-cache \
	wp-mail-smtp duplicate-post redirection updraftplus all-in-one-wp-migration limit-login-attempts-reloaded \
	regenerate-thumbnails tinymce-advanced health-check query-monitor cookie-notice wp-super-cache autoptimize \
	loco-translate; do
	wp plugin install "$p" --path="$P" --quiet || wp plugin install "$p" --path="$P" --quiet ||
		wp plugin install "$p" --path="$P" --quiet
done
wp language core install fr_FR --activate --path="$P" --quiet
wp language plugin install --all fr_FR --path="$P" --quiet || true
wp language theme install --all fr_FR --path="$P" --quiet || true`
	if res, err := runAs(ctx, "sh", "-c", many); err != nil || res.exit != 0 {
		t.Fatalf("site with many plugins: %v %s", err, res.stderr)
	}
	timeScan("a site with 22 plugins, in French", "~/sites/many")
	if os.Getenv("CASES") == "none" {
		return
	}

	// Copies of the plain site, each with its own tables in the plain site's database.
	sites := env("SITES", 20)
	setup := fmt.Sprintf(`set -e
for i in $(seq 1 %d); do
	P=~/sites/tp-$i
	rm -rf "$P"; cp -R ~/sites/plain "$P"; rm -rf "$P/node_modules"
	wp config set table_prefix "tp${i}_" --path="$P" --quiet
	wp core install --path="$P" --url="https://tp-$i.example.test" --title="Copy $i" --admin_user=admin \
		--admin_password=fixture-only --admin_email=admin@example.test --skip-email --quiet
done`, sites)
	if res, err := runAs(ctx, "sh", "-c", setup); err != nil || res.exit != 0 {
		t.Fatalf("copies: %v %s", err, res.stderr)
	}
	t.Cleanup(func() { _, _ = runAs(ctx, "sh", "-c", "rm -rf ~/sites/tp-*") })

	// As the workspace scans a node: 8 sites at a time, the host's limit on its commands.
	scanAll := func(max, accounts, parallel int) (time.Duration, int) {
		// Each account its own shared connection, as different hosts have (sshd takes 10 sessions
		// on one connection by default, MaxSessions), kept 60 seconds as the app does; one limiter
		// for all, which paces the connections opening (ADR-0032).
		limiter := sshx.NewLimiter().KeepOpen(time.Minute)
		scanners := make([]scan.Scanner, accounts)
		for i := range scanners {
			r := a.runner(t, known)
			r.Options.ControlPersist = time.Minute
			scanners[i] = scan.Scanner{Runner: r, Limiter: limiter, Now: timeNow}
		}
		slots := make(chan struct{}, parallel)
		var wg sync.WaitGroup
		var mu sync.Mutex
		failed := 0
		start := time.Now()
		for i := 1; i <= sites; i++ {
			j := job(fmt.Sprintf("~/sites/tp-%d", i), fmt.Sprintf("account-%d", i%accounts), max)
			s := scanners[i%accounts]
			wg.Go(func() {
				slots <- struct{}{}
				defer func() { <-slots }()
				if res, _ := s.Site(ctx, j, nil); res.Outcome != model.ScanCompleted {
					mu.Lock()
					failed++
					mu.Unlock()
					t.Logf("  %s: %s %s", j.Site.Path, res.Outcome, res.Error)
				}
			})
		}
		wg.Wait()
		return time.Since(start), failed
	}
	for _, c := range runCases(os.Getenv("CASES")) {
		took, failed := scanAll(c.max, c.accounts, c.parallel)
		t.Logf("%d sites, %d command(s) at a time on %d account(s), %d sites at a time: %s, %s a site, %d failed", sites, c.max,
			c.accounts, c.parallel, took.Round(100*time.Millisecond), (took / time.Duration(sites)).Round(10*time.Millisecond), failed)
	}
}

type runCase struct{ max, accounts, parallel int }

// runCases reads "2x4x8,2x10x16": commands at a time, accounts, sites at a time; by default, the
// app's limits on one account and on four, with fewer and more commands at a time.
func runCases(list string) []runCase {
	if list == "none" {
		return nil
	}
	if list == "" {
		return []runCase{{1, 1, 16}, {2, 1, 16}, {4, 1, 16}, {2, 4, 16}}
	}
	var cases []runCase
	for _, c := range strings.Split(list, ",") {
		var r runCase
		if _, err := fmt.Sscanf(c, "%dx%dx%d", &r.max, &r.accounts, &r.parallel); err == nil {
			cases = append(cases, r)
		}
	}
	return cases
}
