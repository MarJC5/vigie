// Package addontest checks, from an add-on's own tests, that it keeps Vigie's contract: its
// description reads back and names the program, and a run writes an export that holds together,
// readable by its owner only. An add-on that drifts from Vigie fails its tests, not the user's
// import.
package addontest

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MarJC5/vigie/addon/export"
	"github.com/MarJC5/vigie/addon/info"
)

// Build builds the add-on's main package (a path go build takes, such as "./cmd/vigie-example")
// into a temporary folder and returns the program, named after the package's last element.
func Build(t testing.TB, pkg string) string {
	t.Helper()
	program := filepath.Join(t.TempDir(), filepath.Base(pkg))
	cmd := exec.Command("go", "build", "-o", program, pkg)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build %s: %v\n%s", pkg, err, out)
	}
	return program
}

// Info runs the program with info.InfoFlag and returns its description; the test fails when the
// program fails, or the description does not hold together or names another program.
func Info(t testing.TB, program string) info.Info {
	t.Helper()
	stdout, stderr, err := run(t, program, info.InfoFlag)
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", program, info.InfoFlag, err, stderr)
	}
	i, err := info.Read(bytes.NewReader(stdout))
	if err != nil {
		t.Fatalf("%s %s: %v", program, info.InfoFlag, err)
	}
	if want := strings.TrimPrefix(filepath.Base(program), "vigie-"); i.Name != want {
		t.Errorf("description names %q, the program is vigie-%s", i.Name, want)
	}
	return i
}

// Export runs a source as Vigie does, with the values of its settings, and returns what it wrote
// and its report (its standard error); the test fails when it fails, writes no export, one that
// does not hold together, or one others can read.
func Export(t testing.TB, program string, values map[string]string) (export.File, string) {
	t.Helper()
	i := Info(t, program)
	if i.Kind != info.KindSource {
		t.Fatalf("%s is a %s, not a source", program, i.Kind)
	}
	path := filepath.Join(t.TempDir(), "export.json")
	args, err := i.Args(path, values)
	if err != nil {
		t.Fatal(err)
	}
	_, stderr, err := run(t, program, args...)
	if err != nil {
		t.Fatalf("%s: %v\n%s", program, err, stderr)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("%s wrote no export: %v\n%s", program, err, stderr)
	}
	if st.Mode().Perm()&0o077 != 0 {
		t.Errorf("export readable by others: %v", st.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f, err := export.Read(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("%s wrote an export Vigie refuses: %v", program, err)
	}
	return f, string(stderr)
}

// run runs the program as Vigie does: no shell, RunTimeout at most.
func run(t testing.TB, program string, args ...string) ([]byte, []byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), info.RunTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, program, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}
