// Package addons finds the add-ons installed on the PATH, reads what they say about themselves,
// and runs a source the way the contract of github.com/MarJC5/vigie/addon says: never through a
// shell, each setting its own argument, info.RunTimeout at most, its export read then deleted.
// It knows no add-on by name.
package addons

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/MarJC5/vigie/addon/export"
	"github.com/MarJC5/vigie/addon/info"
)

// Prefix starts the name of every add-on's program.
const Prefix = "vigie-"

// infoTimeout bounds the answer to --vigie-info, which reads nothing.
const infoTimeout = 10 * time.Second

// maxReport bounds what is kept of a run's standard error.
const maxReport = 64 << 10

var (
	// ErrNotASource is returned when a destination is asked to run from the window.
	ErrNotASource = errors.New("not a source")
	// ErrTimedOut is returned when a run takes longer than info.RunTimeout.
	ErrTimedOut = errors.New("the add-on did not finish in time")
	// ErrNoExport is returned when a source ends without writing its export.
	ErrNoExport = errors.New("the add-on wrote no export")
)

// namePattern is the name of a program, after the prefix: no path, no option.
var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Installed is an add-on found on the PATH: its program, and what it says about itself, or why it
// said nothing Vigie can use.
type Installed struct {
	Name string // after the prefix: "example" for vigie-example
	Path string
	Info info.Info
	Err  error
}

// Find lists the add-ons of the PATH given (a list of folders, as $PATH), by name: the first
// program of each name, in the order of the folders, as a shell would run it. Folders that are not
// absolute are skipped, as Go's exec.LookPath refuses a program of the current folder (ErrDot).
// Each one is asked for its description.
func Find(ctx context.Context, path string) []Installed {
	seen := map[string]bool{}
	var found []Installed
	for _, dir := range filepath.SplitList(path) {
		if !filepath.IsAbs(dir) {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name, ok := strings.CutPrefix(e.Name(), Prefix)
			if !ok || !namePattern.MatchString(name) || seen[name] {
				continue
			}
			program := filepath.Join(dir, e.Name())
			if st, err := os.Stat(program); err != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0o111 == 0 {
				continue
			}
			seen[name] = true
			found = append(found, Installed{Name: name, Path: program})
		}
	}
	slices.SortFunc(found, func(a, b Installed) int { return strings.Compare(a.Name, b.Name) })
	for i := range found {
		found[i].Info, found[i].Err = describe(ctx, found[i])
	}
	return found
}

// describe runs the program with info.InfoFlag; its description must name it.
func describe(ctx context.Context, a Installed) (info.Info, error) {
	ctx, cancel := context.WithTimeout(ctx, infoTimeout)
	defer cancel()
	stdout, _, err := run(ctx, a.Path, info.InfoFlag)
	if err != nil {
		return info.Info{}, err
	}
	i, err := info.Read(bytes.NewReader(stdout))
	if err != nil {
		return info.Info{}, err
	}
	if i.Name != a.Name {
		return info.Info{}, fmt.Errorf("its description names %q: %w", i.Name, info.ErrInvalid)
	}
	return i, nil
}

// Failed is a run that ended in error, with what the add-on said.
type Failed struct {
	Report string
	Err    error
}

func (f *Failed) Error() string { return f.Err.Error() }
func (f *Failed) Unwrap() error { return f.Err }

// Run runs a source with the values of its settings, and returns the export it wrote and its
// report (its standard error). The export goes to a folder of its own, readable by the user only,
// deleted once read. A run that fails returns a *Failed with the report.
func Run(ctx context.Context, a Installed, values map[string]string) (export.File, string, error) {
	if a.Err != nil {
		return export.File{}, "", a.Err
	}
	if a.Info.Kind != info.KindSource {
		return export.File{}, "", ErrNotASource
	}
	dir, err := os.MkdirTemp("", "vigie-addon-")
	if err != nil {
		return export.File{}, "", err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "export.json")
	args, err := a.Info.Args(path, values)
	if err != nil {
		return export.File{}, "", err
	}
	ctx, cancel := context.WithTimeout(ctx, info.RunTimeout)
	defer cancel()
	_, stderr, err := run(ctx, a.Path, args...)
	report := string(stderr)
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return export.File{}, report, &Failed{Report: report, Err: ErrTimedOut}
	case err != nil:
		return export.File{}, report, &Failed{Report: report, Err: err}
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return export.File{}, report, &Failed{Report: report, Err: ErrNoExport}
	}
	if err != nil {
		return export.File{}, report, err
	}
	f, err := export.Read(bytes.NewReader(data))
	if err != nil {
		return export.File{}, report, &Failed{Report: report, Err: err}
	}
	return f, report, nil
}

// run runs a program without a shell, its standard input empty, and keeps maxReport of its
// standard error.
func run(ctx context.Context, program string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, program, args...)
	// A program that leaves a child holding its pipes is not waited for long.
	cmd.WaitDelay = time.Second
	var stdout bytes.Buffer
	stderr := limited{max: maxReport}
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.buf.Bytes(), err
}

// limited keeps the first max bytes written to it, and drops the rest.
type limited struct {
	buf bytes.Buffer
	max int
}

func (l *limited) Write(p []byte) (int, error) {
	if room := l.max - l.buf.Len(); room > 0 {
		l.buf.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}
