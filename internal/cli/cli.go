// Package cli parses Vigie's command line and runs the commands that need no window.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/MarJC5/vigie/internal/buildinfo"
	"github.com/MarJC5/vigie/internal/i18n"
	"github.com/MarJC5/vigie/internal/workspace"
)

// Exit statuses, so scripts and cron can act on a scan.
const (
	exitOK        = 0
	exitAttention = 1
	exitError     = 3 // 2 is kept for "urgent" (known vulnerabilities)
)

// Options are the parsed command line.
type Options struct {
	DataDir string   // overrides the per-user configuration directory
	Command []string // empty: open the window
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
	Out, Err io.Writer
	Lang     string
	Texts    *i18n.Catalogs
	// Open opens the user's data; only the commands that need it call it.
	Open func(ctx context.Context) (*workspace.Workspace, error)
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
	case "tree", "scan", "discover":
	default:
		fmt.Fprintf(env.Err, "vigie: %s\n\n%s", env.t("cli.unknownCommand", map[string]any{"command": name}), env.t("cli.usage", nil))
		return exitError
	}

	args, asJSON, err := commandArgs(rest)
	if err != nil {
		return env.fail(env.t("cli.unknownOption", map[string]any{"option": err.Error()}))
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
	switch name {
	case "tree":
		return tree(ctx, env, ws, asJSON)
	case "scan":
		return scanPath(ctx, env, ws, path, asJSON)
	}
	return discoverHost(ctx, env, ws, path, asJSON)
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
