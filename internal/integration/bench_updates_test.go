//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/sshx"
)

// The fixtures of the updates site behave as the update tests need them to.
func TestUpdateBench(t *testing.T) {
	ctx := context.Background()

	t.Run("an update offered by the plugin itself shows only with the plugins loaded", func(t *testing.T) {
		without := previewPlugins(t, "--skip-plugins", "--skip-themes")
		with := previewPlugins(t, "--skip-themes")
		if !slices.Equal(without, []string{"akismet"}) || !slices.Equal(with, []string{"akismet", "fixture-breaks", "fixture-missing", "fixture-pro"}) {
			t.Fatalf("updates without the plugins %v, with them %v", without, with)
		}
	})

	t.Run("the site answers over HTTP by its name", func(t *testing.T) {
		if status, _ := getSite(t, "updates.example.test", "/"); status != http.StatusOK {
			t.Fatalf("home page: status %d", status)
		}
	})

	t.Run("an account without the database dump tools", func(t *testing.T) {
		a := sshAccess(t)
		target := sshx.Target{Hostname: a.host, User: "nodump", Port: a.port, KeyPath: a.key}
		res, err := a.runner(t, a.trusted(t)).Run(ctx, target,
			"command -v mariadb-dump mysqldump || echo no-dump; command -v tar gzip wp php >/dev/null && echo tools")
		if err != nil {
			t.Fatal(err)
		}
		if out := string(res.Stdout); !strings.Contains(out, "no-dump") || !strings.Contains(out, "tools") {
			t.Fatalf("nodump: %q (stderr %q)", out, res.Stderr)
		}
	})

	t.Run("the breaking update gives WordPress's critical error page", func(t *testing.T) {
		t.Cleanup(func() {
			// Version 1.0.0 back, and an empty debug.log, for the next tests.
			res, err := runAs(ctx, "sh", "-c", "wp plugin install /var/www/html/fixture-packages/fixture-breaks-1.0.0.zip"+
				" --force --activate --skip-plugins --skip-themes --path=sites/updates --quiet"+
				" && : > sites/updates/wp-content/debug.log")
			if err != nil || res.exit != 0 {
				t.Errorf("restore fixture-breaks: %v %s", err, res.stderr)
			}
		})
		res, err := runAs(ctx, "wp", "plugin", "update", "fixture-breaks", "--skip-themes", "--path=sites/updates")
		if err != nil || res.exit != 0 {
			t.Fatalf("update: %v, exit %d, %s%s", err, res.exit, res.stdout, res.stderr)
		}
		// PHP's OPcache keeps the old code until it checks the files again (opcache.revalidate_freq,
		// 2 s in the image): the error shows a moment after the update.
		var status int
		var body string
		for end := time.Now().Add(15 * time.Second); time.Now().Before(end); time.Sleep(500 * time.Millisecond) {
			if status, body = getSite(t, "updates.example.test", "/"); status == http.StatusInternalServerError {
				break
			}
		}
		if status != http.StatusInternalServerError || !strings.Contains(body, `id="error-page"`) || !strings.Contains(body, "wp-die-message") {
			t.Fatalf("home page after the update: status %d\n%.400s", status, body)
		}
		log, err := runAs(ctx, "cat", "sites/updates/wp-content/debug.log")
		if err != nil || !strings.Contains(log.stdout, "PHP Fatal error") || !strings.Contains(log.stdout, "fixture_breaks_missing_function") {
			t.Fatalf("debug.log: %v %q", err, log.stdout)
		}
	})
}

// previewPlugins lists, sorted, the plugins `plugin update --dry-run` would update on the updates site.
func previewPlugins(t *testing.T, flags ...string) []string {
	t.Helper()
	args := append([]string{"wp", "plugin", "update", "--all", "--dry-run", "--format=json", "--path=sites/updates"}, flags...)
	res, err := runAs(context.Background(), args...)
	if err != nil || res.exit != 0 {
		t.Fatalf("%v: %v, exit %d, %s", args, err, res.exit, res.stderr)
	}
	var items []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &items); err != nil {
		t.Fatalf("%v: %q: %v", args, res.stdout, err)
	}
	names := []string{}
	for _, i := range items {
		names = append(names, i.Name)
	}
	slices.Sort(names)
	return names
}

// getSite asks the account's web server for a fixture site by its name.
func getSite(t *testing.T, name, path string) (int, string) {
	t.Helper()
	ctx := context.Background()
	host, err := account.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := account.MappedPort(ctx, "80/tcp")
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+host+":"+port.Port()+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = name
	client := http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, string(body)
}
