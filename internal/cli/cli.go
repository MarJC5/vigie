// Package cli parses Vigie's command line and runs the commands that need no window.
package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/MarJC5/vigie/internal/buildinfo"
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

const usage = `Usage: vigie [--data-dir <dir>] [command]

Without a command, Vigie opens its window.

Commands:
  version   Print the version
  help      Print this help
`

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

// Run runs a command and returns the process exit code.
func Run(opts Options, out, errOut io.Writer) int {
	switch opts.Command[0] {
	case "version":
		fmt.Fprintln(out, "vigie", buildinfo.Version())
		return 0
	case "help", "--help", "-h":
		fmt.Fprint(out, usage)
		return 0
	}
	fmt.Fprintf(errOut, "vigie: unknown command %q\n\n%s", opts.Command[0], usage)
	return 2
}
