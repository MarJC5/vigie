package cli_test

import (
	"bytes"
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

const errorsRoot = "/home/u/shop"

// errorsEnv stores Shop/shop.example.test, whose WordPress writes its log, and
// Shop/blog.example.test, whose WordPress writes none; the answers of their servers are given.
func errorsEnv(t *testing.T, shopLog string) func(context.Context) (*workspace.Workspace, error) {
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
	set := func(line string, err error, res sshx.Result) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		fake.Set(line, res)
	}
	ok := func(out string) sshx.Result { return sshx.Result{Stdout: []byte(out)} }
	notDefined := sshx.Result{ExitCode: 1, Stderr: []byte("Error: The constant is not defined in the 'wp-config.php' file.\n")}
	for _, s := range []struct{ path, root string }{{shop.Path, errorsRoot}, {blog.Path, "/home/u/blog"}} {
		line, err := remote.RealRoot(s.path)
		set(line, err, ok(s.root+"\n"))
	}
	line, err := wpcli.ConfigConstant("WP_DEBUG").Remote("wp", shop.Path)
	set(line, err, ok("true\n"))
	line, err = wpcli.ConfigConstant("WP_DEBUG_LOG").Remote("wp", shop.Path)
	set(line, err, ok("true\n"))
	if shopLog != "" {
		line, err = remote.ErrorLines(errorsRoot + "/wp-content/debug.log")
		set(line, err, ok(shopLog))
	}
	for _, name := range []string{"WP_DEBUG", "WP_DEBUG_LOG"} {
		line, err = wpcli.ConfigConstant(name).Remote("wp", blog.Path)
		set(line, err, notDefined)
	}
	line, err = remote.ErrorLines("/home/u/blog/wp-content/debug.log")
	set(line, err, ok(""))
	ws := workspace.New(db, workspace.Deps{Runner: fake, Now: time.Now})
	return func(context.Context) (*workspace.Workspace, error) { return ws, nil }
}

const shopLog = "4012345\n" +
	"[03-Nov-2026 02:14:00 UTC] PHP Fatal error:  Uncaught Error: Call to undefined function wc_get_order() in " + errorsRoot + "/wp-content/themes/astra-child/functions.php:42\n" +
	"[09-Nov-2026 08:03:00 UTC] PHP Fatal error:  Uncaught Error: Call to undefined function wc_get_order() in " + errorsRoot + "/wp-content/themes/astra-child/functions.php:42\n" +
	"[05-Nov-2026 09:00:00 UTC] PHP Warning:  Could not send to marie@example.test in " + errorsRoot + "/wp-content/plugins/forms/mail.php on line 7\n"

func runErrors(t *testing.T, lang string, open func(context.Context) (*workspace.Workspace, error), args ...string) (int, string, string) {
	t.Helper()
	opts, err := cli.Parse(append([]string{"errors"}, args...))
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := cli.Run(context.Background(), opts, cli.Env{Out: &out, Err: &errOut, Lang: lang, Texts: texts(t), Open: open})
	return code, out.String(), errOut.String()
}

func local(t time.Time) string { return t.Local().Format("02.01.2006 15:04") }

func TestErrorsOfASite(t *testing.T) {
	code, out, errOut := runErrors(t, "en", errorsEnv(t, shopLog), "Shop/shop.example.test")
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	first, last := time.Date(2026, 11, 3, 2, 14, 0, 0, time.UTC), time.Date(2026, 11, 9, 8, 3, 0, 0, time.UTC)
	for _, want := range []string{
		"Shop/shop.example.test: wp-content/debug.log\n",
		", back to " + local(first) + ": the last megabyte of 4\u00a0MB.\n", // a size keeps its unit on its line
		"1 error, 1 warning, 3 lines read.\n",                               // groups, and the lines read
		"Errors\n  Fatal error in the theme astra-child  2 times\n",
		"    wp-content/themes/astra-child/functions.php, line 42 · " + local(first) + " to " + local(last) + "\n",
		"    | Uncaught Error: Call to undefined function wc_get_order()\n",
		"Warnings\n  Warning in the plugin forms  once\n",
		"    | Could not send to [email]\n",
		"What changed just before (in the 7 days before each error, not necessarily the cause):\n  Vigie noted nothing in the 7 days before these errors.\n",
		"a lead, not a verdict",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "marie@") || strings.Contains(out, errorsRoot) {
		t.Errorf("a masked text crossed:\n%s", out)
	}
}

func TestErrorsAsJSON(t *testing.T) {
	code, out, _ := runErrors(t, "en", errorsEnv(t, shopLog), "Shop/shop.example.test", "--json")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	var got struct {
		Path, State, File string
		Size              int64
		Groups            []struct {
			Type, Kind, Slug, Message string
			Count                     int
		}
		Before []any
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got.Path != "Shop/shop.example.test" || got.State != "read" || got.File != "wp-content/debug.log" || got.Size != 4012345 ||
		len(got.Groups) != 2 || got.Groups[0].Kind != "theme" || got.Groups[0].Count != 2 || got.Before == nil {
		t.Errorf("JSON = %+v", got)
	}
}

// A site whose WordPress writes no log: said, with how to turn it on by hand, in French too.
func TestErrorsWithoutALog(t *testing.T) {
	code, out, _ := runErrors(t, "fr", errorsEnv(t, ""), "Shop/blog.example.test")
	if code != 0 || !strings.Contains(out, "WordPress n'écrit pas de journal sur ce site") || !strings.Contains(out, "define( 'WP_DEBUG_LOG', true );") {
		t.Errorf("exit %d:\n%s", code, out)
	}
}

func TestErrorsRefused(t *testing.T) {
	open := errorsEnv(t, shopLog)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{nil, "one site at a time"},
		{[]string{"Shop"}, "one site at a time"},
		{[]string{"Shop/nothing.example.test"}, `nothing at "Shop/nothing.example.test"`},
		{[]string{"--tag", "clients"}, `unknown option "--tag"`},
	} {
		if code, _, errOut := runErrors(t, "en", open, tc.args...); code != 3 || !strings.Contains(errOut, tc.want) {
			t.Errorf("errors %q: exit %d, %q; want 3 and %q", tc.args, code, errOut, tc.want)
		}
	}
	// A server that does not answer the read: an error, with exit status 3.
	if code, out, errOut := runErrors(t, "en", errorsEnv(t, ""), "Shop/shop.example.test"); code != 3 || out != "" || errOut == "" {
		t.Errorf("unanswered: exit %d, out %q, err %q", code, out, errOut)
	}
}
