// Package cli parses Vigie's command line and runs the commands that need no window.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"

	"github.com/MarJC5/vigie/internal/addons"
	"github.com/MarJC5/vigie/internal/buildinfo"
	"github.com/MarJC5/vigie/internal/i18n"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/workspace"
)

// Exit statuses, so scripts and cron can act on a scan.
const (
	exitOK        = 0
	exitAttention = 1 // needs attention, or not fully checked
	exitUrgent    = 2
	exitError     = 3
)

// Options are the parsed command line.
type Options struct {
	DataDir string   // overrides the per-user configuration directory
	Command []string // empty: open the window
	AtLogin bool     // started by the system at login: the window opens hidden
}

// OpensWindow tells whether Vigie opens its window rather than running a command.
func (o Options) OpensWindow() bool {
	return len(o.Command) == 0
}

// Parse reads the arguments that follow the program name.
func Parse(args []string) (Options, error) {
	var opts Options
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--data-dir":
			if i+1 >= len(args) {
				return opts, errors.New("--data-dir needs a directory")
			}
			i++
			opts.DataDir = args[i]
		case strings.HasPrefix(arg, "--data-dir="):
			opts.DataDir = strings.TrimPrefix(arg, "--data-dir=")
		case arg == "--at-login":
			opts.AtLogin = true
		case strings.HasPrefix(arg, "-psn_"):
			// Process serial number added by the macOS launcher.
		default:
			opts.Command = append(opts.Command, arg)
		}
	}
	return opts, nil
}

// Env is what the commands need from outside; main fills it, tests replace it.
type Env struct {
	In       io.Reader // answers the confirmation of an update
	Out, Err io.Writer
	Lang     string
	Texts    *i18n.Catalogs
	// Open opens the user's data; only the commands that need it call it.
	Open func(ctx context.Context) (*workspace.Workspace, error)
	// Terminal: where the command runs; its progress shows only in an interactive one (ADR-0037).
	Terminal Terminal
}

func (e Env) t(key string, vars map[string]any) string {
	return e.Texts.T(e.Lang, key, vars)
}

// fail prints an error and returns the error status.
func (e Env) fail(message string) int {
	fmt.Fprintln(e.Err, "vigie:", message)
	return exitError
}

// Run runs a command and returns the process exit status.
func Run(ctx context.Context, opts Options, env Env) int {
	name, rest := opts.Command[0], opts.Command[1:]
	switch name {
	case "version":
		fmt.Fprintln(env.Out, "vigie", buildinfo.Version())
		return exitOK
	case "help", "--help", "-h":
		fmt.Fprint(env.Out, env.t("cli.usage", nil))
		return exitOK
	case "export", "import":
		return runData(ctx, env, name, rest)
	case "tree", "scan", "discover", "update":
	default:
		// Not a command of Vigie: an add-on's, when vigie-<name> is on the PATH.
		if code, ok := runAddOn(env, name, rest); ok {
			return code
		}
		fmt.Fprintf(env.Err, "vigie: %s\n\n%s", env.t("cli.unknownCommand", map[string]any{"command": name}), env.t("cli.usage", nil))
		return exitError
	}

	// --yes answers the confirmation of an update, for scripts; no other command asks.
	yes := false
	if name == "update" {
		rest, yes = withoutFlag(rest, "--yes")
	}
	// --check-files compares the files with wordpress.org's even when it was done less than a day ago.
	checkFiles := false
	if name == "scan" {
		rest, checkFiles = withoutFlag(rest, "--check-files")
	}
	// --tag acts on the sites with a tag, wherever they are, in place of a path.
	tagged := ""
	if name == "tree" || name == "scan" || name == "update" {
		var err error
		if rest, tagged, err = withValue(rest, "--tag"); err != nil {
			return env.fail(env.t("cli.needsValue", map[string]any{"option": "--tag"}))
		}
	}
	args, asJSON, err := commandArgs(rest)
	if err != nil {
		return env.fail(env.t("cli.unknownOption", map[string]any{"option": err.Error()}))
	}
	if tagged != "" && len(args) > 0 {
		return env.fail(env.t("cli.tagAndPath", nil))
	}
	if name == "discover" && len(args) != 1 {
		return env.fail(env.t("cli.needsHost", nil))
	}
	ws, err := env.Open(ctx)
	if err != nil {
		return env.fail(err.Error())
	}
	defer ws.Close()
	path := strings.Join(args, " ")
	on := target{path: path, tag: tagged}
	switch name {
	case "tree":
		return tree(ctx, env, ws, tagged, asJSON)
	case "scan":
		return scanTarget(ctx, env, ws, on, workspace.ScanOptions{CheckFiles: checkFiles, By: model.RunByCLI}, asJSON)
	case "update":
		return updateTarget(ctx, env, ws, on, asJSON, yes)
	}
	return discoverHost(ctx, env, ws, path, asJSON)
}

// addOnName is the name of an add-on: no path, no option.
var addOnName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// runAddOn runs vigie-<name> from the PATH with the arguments as they are, as Git runs its external
// commands, and returns its exit status; ok is false when there is no such program. Never through a
// shell, never a program of the current folder (exec.ErrDot).
func runAddOn(env Env, name string, args []string) (int, bool) {
	if !addOnName.MatchString(name) {
		return 0, false
	}
	program, err := exec.LookPath(addons.Prefix + name)
	if err != nil {
		return 0, false
	}
	cmd := exec.Command(program, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = env.In, env.Out, env.Err
	err = cmd.Run()
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return exit.ExitCode(), true
	}
	if err != nil {
		return env.fail(err.Error()), true
	}
	return exitOK, true
}

// commandArgs separates --json from the path; any other option is refused (returned as the error).
func commandArgs(args []string) (rest []string, asJSON bool, err error) {
	for _, a := range args {
		switch {
		case a == "--json":
			asJSON = true
		case strings.HasPrefix(a, "-"):
			return nil, false, errors.New(a)
		default:
			rest = append(rest, a)
		}
	}
	return rest, asJSON, nil
}

// withValue removes an option and the value after it from the arguments, and returns the value;
// "" when the option is not there, an error when it has no value.
func withValue(args []string, option string) ([]string, string, error) {
	var rest []string
	value := ""
	for i := 0; i < len(args); i++ {
		if args[i] != option {
			rest = append(rest, args[i])
			continue
		}
		if i+1 == len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
			return nil, "", errors.New(option)
		}
		i++
		value = args[i]
	}
	return rest, value, nil
}

// target is what a command acts on: a path, everything when it is empty, or the sites with a tag.
type target struct{ path, tag string }

// errUnknownTag: no site has the tag given.
var errUnknownTag = errors.New("no site has this tag")

func (t target) node(tree *model.Tree) (model.Node, error) {
	switch {
	case t.tag != "":
		if len(tree.SitesUnder(model.TagNode(t.tag))) == 0 {
			return model.Node{}, errUnknownTag
		}
		return model.TagNode(t.tag), nil
	case t.path == "":
		return model.Root, nil
	}
	return tree.Resolve(t.path)
}

// explain turns an error of the target into a sentence for the user.
func (t target) explain(env Env, err error) string {
	if errors.Is(err, errUnknownTag) {
		return env.t("cli.unknownTag", map[string]any{"tag": t.tag})
	}
	return explain(env, t.path, err)
}

// withoutFlag removes a flag from the arguments and tells whether it was there.
func withoutFlag(args []string, flag string) ([]string, bool) {
	var rest []string
	found := false
	for _, a := range args {
		if a == flag {
			found = true
			continue
		}
		rest = append(rest, a)
	}
	return rest, found
}
