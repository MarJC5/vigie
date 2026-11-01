//go:build integration

package integration_test

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/wpcli"
)

var big = flag.Bool("big", false, "grow a network of thousands of subsites and read it with the subsites script (slow)")

// The steps of the network's growth: its subsites besides the main site.
var bigSteps = []int{100, 250, 500, 1000, 2000, 4000, 8000}

// TestSubsitesScriptOnABigNetwork grows a network in the bench, step by step, and reads it each
// time with the subsites script over SSH, within the app's limit for one command (ADR-0029): how
// long it takes, how much it prints, how much memory PHP uses, up to the first step that fails.
// Each subsite has an administrator of its own; one in three has a plugin active.
func TestSubsitesScriptOnABigNetwork(t *testing.T) {
	if !*big {
		t.Skip("run with -big: it grows a network of thousands of subsites")
	}
	ctx := context.Background()
	root := sites + "/big"
	setup := `set -e
P=` + root + `
rm -rf "$P"; mkdir -p "$P"; cp -R /usr/src/wordpress/. "$P"
C=` + sites + `/plain
wp config create --path="$P" --dbname=fixture_big --dbuser="$(wp config get DB_USER --path="$C")" \
	--dbpass="$(wp config get DB_PASSWORD --path="$C")" --dbhost="$(wp config get DB_HOST --path="$C")" --skip-check --quiet
wp db drop --yes --path="$P" --quiet 2>/dev/null || true
wp db create --path="$P" --quiet
wp core multisite-install --path="$P" --url=https://big.example.test --title=Big --admin_user=admin \
	--admin_password=fixture-only --admin_email=admin@example.test --skip-email --quiet
php -r 'echo "php: memory_limit=", ini_get("memory_limit"), "\n";'
wp eval 'echo "under WP-CLI: memory_limit=", ini_get("memory_limit"), "\n";' --path="$P"`
	res, err := runAs(ctx, "sh", "-c", setup)
	if err != nil || res.exit != 0 {
		t.Fatalf("setup: %v %s", err, res.stderr)
	}
	t.Logf("PHP on the command line: %s", strings.TrimSpace(res.stdout))

	a := sshAccess(t)
	runner := a.runner(t, a.trusted(t))
	runner.Timeout = 2 * time.Minute // the app's limit for one command (workspace.commandTimeout)
	line, err := wpcli.Subsites().Remote("wp", "~/sites/big")
	if err != nil {
		t.Fatal(err)
	}
	// The script as Vigie sends it, then its peak memory on standard error, for this test only.
	script := append(wpcli.SubsitesScript(), []byte("\nfwrite( STDERR, 'peak=' . memory_get_peak_usage( true ) );\n")...)

	// One retry when ssh's shared connection fails its handshake, said in the log: the growth leaves
	// minutes without a command, and that failure is looked at apart.
	send := func(remote string) (sshx.Result, error) {
		res, err := runner.RunInput(ctx, a.target(a.key), remote, script)
		if err != nil && strings.Contains(err.Error(), "mux") {
			t.Logf("shared connection failed (%v), tried again", err)
			res, err = runner.RunInput(ctx, a.target(a.key), remote, script)
		}
		return res, err
	}

	from := 1
	for _, size := range bigSteps {
		grow := time.Now()
		code := fmt.Sprintf(`for ( $i = %d; $i <= %d; $i++ ) {
	$user = wp_create_user( "admin$i", "fixture-only", "admin$i@example.test" );
	$blog = wpmu_create_blog( "big.example.test", "/s$i/", "S$i", $user, array( "public" => 1 ), get_current_network_id() );
	if ( 0 === $i %% 3 ) {
		switch_to_blog( $blog );
		update_option( "active_plugins", array( "hello.php" ) );
		restore_current_blog();
	}
}`, from, size)
		// Growing is the bench's work, not the script's: without PHP's memory limit.
		if res, err := runAs(ctx, "php", "-d", "memory_limit=-1", "/usr/local/bin/wp", "eval", code, "--path="+root); err != nil || res.exit != 0 {
			t.Fatalf("growing to %d subsites: %v %s", size, err, res.stderr)
		}
		from = size + 1
		grown := time.Since(grow)

		start := time.Now()
		out, err := send(line)
		took := time.Since(start)
		if err != nil || out.ExitCode != 0 {
			t.Errorf("%d subsites: the script failed after %s: %v, exit %d, %s", size, took.Round(time.Second), err, out.ExitCode,
				tail(string(out.Stderr), 300))
			return
		}
		network, err := wpcli.ParseSubsites(out)
		if err != nil {
			t.Errorf("%d subsites: %v", size, err)
			return
		}
		admins, plugins := 0, 0
		for _, s := range network.Sites {
			admins += len(s.Admins)
			plugins += len(s.Plugins)
		}
		_, peak, _ := strings.Cut(string(out.Stderr), "peak=")
		// WP-CLI lifts PHP's memory limit (-1 on the bench, whatever php -d sets): the peak is what a
		// host's own cap on a process would have to allow.
		t.Logf("%5d subsites: read in %6s, %7d bytes printed, PHP peak %s MB; %d admins, %d active plugins (grown in %s)",
			size, took.Round(10*time.Millisecond), len(out.Stdout), megabytes(peak), admins, plugins, grown.Round(time.Second))
		if len(network.Sites) != size+1 {
			t.Errorf("%d subsites: %d read, want %d with the main site", size, len(network.Sites), size+1)
			return
		}
	}
}

func megabytes(bytes string) string {
	var n float64
	if _, err := fmt.Sscan(strings.TrimSpace(bytes), &n); err != nil {
		return "?"
	}
	return fmt.Sprintf("%.0f", n/1e6)
}

func tail(s string, n int) string {
	if len(s) > n {
		return "…" + s[len(s)-n:]
	}
	return s
}
