package discover_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/MarJC5/vigie/internal/discover"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/wpcli"
)

type recorded struct {
	Exit           int
	Stdout, Stderr string
}

func read(t *testing.T, path string) recorded {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var r recorded
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func result(r recorded) sshx.Result {
	return sshx.Result{Stdout: []byte(r.Stdout), Stderr: []byte(r.Stderr), ExitCode: r.Exit}
}

func TestParseFindOnTheBenchAccount(t *testing.T) {
	roots, truncated := discover.ParseFind([]byte(read(t, "testdata/recorded/find.json").Stdout))
	want := []string{
		"~/sites/broken", "~/sites/network", "~/sites/offline", "~/sites/outdated",
		"~/sites/parent-config/public", "~/sites/plain",
	}
	if !reflect.DeepEqual(roots, want) || truncated {
		t.Fatalf("ParseFind() = %q, %v; want %q (the copy under node_modules pruned)", roots, truncated, want)
	}
}

func TestParseFindIgnoresOtherLinesAndCaps(t *testing.T) {
	var out strings.Builder
	out.WriteString("./wp-includes/version.php\n") // WordPress in the home directory itself
	out.WriteString("find: './private': Permission denied\n")
	out.WriteString("./a/wp-includes/version.php.bak\n")
	out.WriteString("../escape/wp-includes/version.php\n")
	for i := range 250 {
		out.WriteString("./many/" + strings.Repeat("x", i%7+1) + "/" + string(rune('a'+i%26)) + "/" + strconv.Itoa(i) + "/wp-includes/version.php\n")
	}
	roots, truncated := discover.ParseFind([]byte(out.String()))
	if len(roots) != discover.MaxCandidates || !truncated {
		t.Fatalf("ParseFind() = %d roots, truncated %v; want %d and truncated", len(roots), truncated, discover.MaxCandidates)
	}
	if !slices.Contains(roots, "~/.") {
		t.Fatalf("home directory install missing: %q", roots[:3])
	}
	for _, r := range roots {
		if strings.Contains(r, "..") || strings.HasSuffix(r, ".bak") {
			t.Fatalf("unexpected root %q", r)
		}
	}
}

func FuzzParseFind(f *testing.F) {
	f.Add([]byte("./sites/a/wp-includes/version.php\n./wp-includes/version.php\n"))
	f.Fuzz(func(t *testing.T, out []byte) {
		roots, _ := discover.ParseFind(out)
		if len(roots) > discover.MaxCandidates {
			t.Fatalf("%d roots", len(roots))
		}
		for _, r := range roots {
			if !strings.HasPrefix(r, "~/") || strings.Contains(r, "\n") {
				t.Fatalf("root %q", r)
			}
			if _, err := wpcli.CoreVersion().Remote("wp", r); err != nil {
				t.Fatalf("root %q cannot be used: %v", r, err)
			}
		}
	})
}

// bench answers the discovery of the bench account with recorded outputs.
func bench(t *testing.T) *sshxtest.Fake {
	t.Helper()
	fake := sshxtest.NewFake()
	fake.Set(discover.FindCommand(), result(read(t, "testdata/recorded/find.json")))
	for fixture, root := range map[string]string{
		"plain": "~/sites/plain", "parent-config": "~/sites/parent-config/public", "network": "~/sites/network",
		"outdated": "~/sites/outdated", "offline": "~/sites/offline", "broken": "~/sites/broken",
	} {
		for name, cmd := range map[string]wpcli.Command{
			"core-is-installed":         wpcli.CoreIsInstalled(false),
			"core-is-installed-network": wpcli.CoreIsInstalled(true),
			"option-home":               wpcli.OptionGet("home"),
			"site-list":                 wpcli.SiteList(),
		} {
			remote, err := cmd.Remote("wp", root)
			if err != nil {
				t.Fatal(err)
			}
			fake.Set(remote, result(read(t, filepath.Join("..", "wpcli", "testdata", "recorded", fixture, name+".json"))))
		}
	}
	return fake
}

func TestDiscoverTheBenchAccount(t *testing.T) {
	existing := []model.Site{{Name: "outdated", Path: "~/sites/outdated/"}}
	d := discover.Discoverer{Runner: bench(t), Limiter: sshx.NewLimiter()}
	found, truncated, err := d.Host(context.Background(), discover.Account{
		Target: sshx.Target{Alias: "bench"}, HostKey: "host-1", Max: 2, WPCLI: "wp",
	}, existing)
	if err != nil || truncated {
		t.Fatalf("Host() error = %v, truncated %v", err, truncated)
	}
	byPath := map[string]discover.Found{}
	for _, f := range found {
		byPath[f.Path] = f
	}
	want := map[string]discover.Found{
		"~/sites/plain":                {Path: "~/sites/plain", State: discover.StateNew, Name: "plain.example.test", URL: "https://plain.example.test"},
		"~/sites/parent-config/public": {Path: "~/sites/parent-config/public", State: discover.StateNew, Name: "parent.example.test", URL: "https://parent.example.test"},
		"~/sites/network": {Path: "~/sites/network", State: discover.StateNew, Name: "network.example.test",
			URL: "https://network.example.test", Multisite: true, Subsites: 4},
		"~/sites/offline":  {Path: "~/sites/offline", State: discover.StateNew, Name: "offline.example.test", URL: "https://offline.example.test"},
		"~/sites/outdated": {Path: "~/sites/outdated", State: discover.StateAlreadyAdded, Name: "outdated"},
	}
	for path, w := range want {
		if got := byPath[path]; got != w {
			t.Errorf("%s:\n got %+v\nwant %+v", path, got, w)
		}
	}
	broken := byPath["~/sites/broken"]
	if broken.State != discover.StateCannotLoad || !strings.Contains(broken.Message, "database") || broken.Name != "broken" {
		t.Errorf("broken = %+v, want cannot-load with WP-CLI's message", broken)
	}
	if len(found) != 6 {
		t.Errorf("Host() found %d installs, want 6", len(found))
	}
}

func TestDiscoverReportsAFailedSearch(t *testing.T) {
	fake := sshxtest.NewFake()
	fake.Set(discover.FindCommand(), sshx.Result{ExitCode: 1, Stderr: []byte("find: unknown predicate `-maxdepth'\n")})
	d := discover.Discoverer{Runner: fake, Limiter: sshx.NewLimiter()}
	_, _, err := d.Host(context.Background(), discover.Account{Target: sshx.Target{Alias: "a"}, HostKey: "h", Max: 2, WPCLI: "wp"}, nil)
	if !errors.Is(err, discover.ErrFind) || !strings.Contains(err.Error(), "maxdepth") {
		t.Fatalf("Host() error = %v, want ErrFind with find's message", err)
	}
}

func TestUnreadableFoldersDoNotStopTheSearch(t *testing.T) {
	fake := sshxtest.NewFake()
	fake.Set(discover.FindCommand(), sshx.Result{ExitCode: 1,
		Stdout: []byte("./www/wp-includes/version.php\n"), Stderr: []byte("find: './private': Permission denied\n")})
	fake.Set(mustRemote(t, wpcli.CoreIsInstalled(false), "~/www"), sshx.Result{ExitCode: 1})
	d := discover.Discoverer{Runner: fake, Limiter: sshx.NewLimiter()}
	found, _, err := d.Host(context.Background(), discover.Account{Target: sshx.Target{Alias: "a"}, HostKey: "h", Max: 2, WPCLI: "wp"}, nil)
	if err != nil || len(found) != 1 || found[0].State != discover.StateNotInstalled {
		t.Fatalf("Host() = %+v, %v; want the install found, not installed", found, err)
	}
}

func mustRemote(t *testing.T, cmd wpcli.Command, path string) string {
	t.Helper()
	remote, err := cmd.Remote("wp", path)
	if err != nil {
		t.Fatal(err)
	}
	return remote
}
