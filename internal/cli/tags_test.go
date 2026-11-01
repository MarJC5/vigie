package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/MarJC5/vigie/internal/cli"
	"github.com/MarJC5/vigie/internal/workspace"
)

// tag gives sites of a workspace a tag, by their paths.
func tag(t *testing.T, ws *workspace.Workspace, tag string, paths ...string) {
	t.Helper()
	ctx := context.Background()
	tree, err := ws.DB().LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		n, err := tree.Resolve(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := ws.SetTags(ctx, n, []string{tag}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTreeOfATag(t *testing.T) {
	ws := benchWorkspace(t)
	tag(t, ws, "Contrat", "Clients/Bench/outdated.example.test", "Clients/Bench/broken")
	code, out := runIn(t, ws, "en", "tree", "--tag", "contrat")
	if code != 0 || !strings.Contains(out, "Clients/Bench/outdated.example.test") || !strings.Contains(out, "Clients/Bench/broken") ||
		strings.Contains(out, "network") {
		t.Fatalf("exit %d, want the two sites tagged only:\n%s", code, out)
	}
	_, out = runIn(t, ws, "en", "tree", "--tag", "Contrat", "--json")
	var nodes []struct{ Path, Kind string }
	if err := json.Unmarshal([]byte(out), &nodes); err != nil || len(nodes) != 2 || nodes[0].Kind != "site" {
		t.Fatalf("tree --tag --json = %+v, %v\n%s", nodes, err, out)
	}
}

func TestScanATag(t *testing.T) {
	ws := benchWorkspace(t)
	tag(t, ws, "Contrat", "Clients/Bench/outdated.example.test", "Clients/Bench/broken")
	_, out := runIn(t, ws, "en", "scan", "--tag", "CONTRAT", "--json")
	var scans []struct{ Path string }
	if err := json.Unmarshal([]byte(out), &scans); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if len(scans) != 2 || scans[0].Path != "Clients/Bench/outdated.example.test" || scans[1].Path != "Clients/Bench/broken" {
		t.Fatalf("scan --tag scanned %+v, want the two sites tagged", scans)
	}
}

func TestUpdateATag(t *testing.T) {
	f := newFake()
	open := updateEnv(t, f)
	ws, err := open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	tag(t, ws, "Contrat", "Shop/blog.example.test")
	opts, err := cli.Parse([]string{"update", "--tag", "contrat"})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	cli.Run(context.Background(), opts, cli.Env{In: strings.NewReader("n\n"), Out: &out, Err: &errOut, Lang: "en", Texts: texts(t), Open: open})
	if !strings.Contains(out.String(), "Shop/blog.example.test") || strings.Contains(out.String(), "Shop/shop.example.test") {
		t.Fatalf("update --tag previewed:\n%s%s", out.String(), errOut.String())
	}
}

func TestTagErrorsExplainThemselves(t *testing.T) {
	ws := benchWorkspace(t)
	tag(t, ws, "Contrat", "Clients/Bench/broken")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"scan", "--tag"}, "--tag needs a value"},
		{[]string{"scan", "Clients/Bench", "--tag", "Contrat"}, "a path or --tag, not both"},
		{[]string{"update", "--tag", "nope", "--yes"}, `no site has the tag "nope"`},
		{[]string{"tree", "--tag", "nope"}, `no site has the tag "nope"`},
	} {
		if code, out := runIn(t, ws, "en", tc.args...); code != 3 || !strings.Contains(out, tc.want) {
			t.Errorf("%q: exit %d, %q; want 3 and %q", tc.args, code, out, tc.want)
		}
	}
	if _, out := runIn(t, ws, "fr", "scan", "--tag", "rien"); !strings.Contains(out, "aucun site ne porte l'étiquette « rien »") {
		t.Errorf("in French: %q", out)
	}
}
