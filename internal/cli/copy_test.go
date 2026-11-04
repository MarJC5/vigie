package cli_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/cli"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/workspace"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// copyEnv stores Shop/shop.example.test, whose account answers a copy of its database, and
// Shop/blog.example.test, whose account answers nothing.
func copyEnv(t *testing.T) (func(context.Context) (*workspace.Workspace, error), string) {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "vigie.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(ctx, store.Migrations(), "test"); err != nil {
		t.Fatal(err)
	}
	host := model.Host{ID: model.NewID(), Name: "Shop", Alias: "shop"}
	shop := model.Site{ID: model.NewID(), HostID: host.ID, Name: "shop.example.test", Path: "~/shop"}
	blog := model.Site{ID: model.NewID(), HostID: host.ID, Name: "blog.example.test", Path: "~/blog", Position: 1}
	for _, err := range []error{db.SaveHost(ctx, host), db.SaveSite(ctx, shop), db.SaveSite(ctx, blog)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	fake := sshxtest.NewFake()
	wp := func(c wpcli.Command) string { line, _ := c.Remote("wp", shop.Path); return line }
	ok := func(out string) sshx.Result { return sshx.Result{Stdout: []byte(out)} }
	fake.Set(wp(wpcli.CopyPaths()), ok(`{"root":"/h/shop","content":"/h/shop/wp-content","plugins":"/h/shop/wp-content/plugins","muPlugins":"/h/shop/wp-content/mu-plugins","themes":"/h/shop/wp-content/themes","uploads":"/h/shop/wp-content/uploads","prefix":"wp_"}`))
	fake.Set(wp(wpcli.CoreVersion()), ok("7.1.3\n"))
	fake.Set(wp(wpcli.DatabaseSize()), ok("12400\n"))
	fake.Set(wp(wpcli.DatabaseTables()), ok(`[{"Name":"wp_posts","Size":"12400 B"}]`))
	fake.Set(remote.CopyTools(), ok("---\n"))
	for _, dir := range []string{"/h/shop/wp-content", "/h/shop"} {
		line, _ := remote.ContentSizes(dir)
		fake.Set(line, ok("---\n8\t.\n"))
	}
	var dump bytes.Buffer
	zw := gzip.NewWriter(&dump)
	zw.Write([]byte("-- MySQL dump\n-- Dump completed on 2026-11-04 10:00:00\n"))
	zw.Close()
	fake.Set(remote.DumpCompressed(wp(wpcli.DatabaseDump(nil))), sshx.Result{Stdout: dump.Bytes(), Stderr: []byte("vigie-dump-exit 0\n")})
	home := t.TempDir()
	ws := workspace.New(db, workspace.Deps{Runner: fake, Home: home, Now: time.Now})
	return func(context.Context) (*workspace.Workspace, error) { return ws, nil }, home
}

func runCopy(t *testing.T, lang string, open func(context.Context) (*workspace.Workspace, error), args ...string) (int, string, string) {
	t.Helper()
	opts, err := cli.Parse(append([]string{"copy"}, args...))
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := cli.Run(context.Background(), opts, cli.Env{Out: &out, Err: &errOut, Lang: lang, Texts: texts(t), Open: open})
	return code, out.String(), errOut.String()
}

// The sites of a host copied, each told as it ends; one that cannot be read is said, and the exit
// status says it too (ADR-0041, point 8).
func TestCopyAHost(t *testing.T) {
	open, home := copyEnv(t)
	code, out, _ := runCopy(t, "fr", open, "Shop", "--content", "database")
	if code != 3 {
		t.Errorf("exit %d, want 3: a site was not copied", code)
	}
	folder := filepath.Join(home, "Documents", "Vigie", "Shop", "shop.example.test")
	if !strings.Contains(out, "Shop/shop.example.test  copié : 1 archive, ") || !strings.Contains(out, "dans "+folder) {
		t.Errorf("out =\n%s", out)
	}
	if !strings.Contains(out, "Shop/blog.example.test  pas copié : ") {
		t.Errorf("the site that cannot be read: out =\n%s", out)
	}
}

func TestCopyJSON(t *testing.T) {
	open, _ := copyEnv(t)
	code, out, _ := runCopy(t, "en", open, "Shop/shop.example.test", "--content", "database", "--json")
	var got []struct {
		Path, Outcome string
		Archives      int
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil || code != 0 || len(got) != 1 || got[0].Outcome != "completed" || got[0].Archives != 1 {
		t.Fatalf("copy --json = %d, %s (%v)", code, out, err)
	}
}

func TestCopyRefusesAnUnknownContent(t *testing.T) {
	open, _ := copyEnv(t)
	code, _, errOut := runCopy(t, "en", open, "Shop", "--content", "everything")
	if code != 3 || !strings.Contains(errOut, "--content is database, content or site, not everything.") {
		t.Fatalf("copy --content everything = %d, %q", code, errOut)
	}
}
