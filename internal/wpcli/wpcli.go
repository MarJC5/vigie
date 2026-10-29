// Package wpcli builds the WP-CLI commands Vigie runs on servers. Commands come from a fixed list
// with checked arguments; nothing is built from free text.
package wpcli

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/MarJC5/vigie/internal/sshx"
)

var (
	ErrNoCommand   = errors.New("no WP-CLI command configured")
	ErrBadPath     = errors.New("invalid WordPress path")
	ErrBadArgument = errors.New("invalid argument")
)

// Command is a WP-CLI command with its arguments.
type Command struct {
	args   []string
	read   bool // read commands skip loading plugins and themes, so a broken plugin does not break them
	global bool // about WP-CLI itself: no WordPress, so no path
	err    error
}

// CoreVersion returns the installed WordPress version.
func CoreVersion() Command {
	return Command{args: []string{"core", "version"}, read: true}
}

// CoreIsInstalled answers through its exit code; network checks for a multisite network.
func CoreIsInstalled(network bool) Command {
	args := []string{"core", "is-installed"}
	if network {
		args = append(args, "--network")
	}
	return Command{args: args, read: true}
}

var optionName = regexp.MustCompile(`^[a-z0-9_][a-z0-9_-]*$`)

// OptionGet reads one option, for example "home".
func OptionGet(name string) Command {
	if !optionName.MatchString(name) {
		return Command{err: fmt.Errorf("option name %q: %w", name, ErrBadArgument)}
	}
	return Command{args: []string{"option", "get", name}, read: true}
}

// Remote returns the quoted command line for the server. wpcli is the configured command, split
// on spaces ("wp", "wp-cli", "php ~/wp-cli.phar"); path is the WordPress root.
func (c Command) Remote(wpcli, path string) (string, error) {
	if c.err != nil {
		return "", c.err
	}
	words := strings.Fields(wpcli)
	if len(words) == 0 {
		return "", ErrNoCommand
	}
	// "php ~/wp-cli.phar": a quoted "~" is not expanded, use the path relative to the home.
	words = trimHome(words)
	if c.global {
		if path != "" {
			return "", fmt.Errorf("%q for a command about WP-CLI itself: %w", path, ErrBadPath)
		}
		return sshx.Quote(append(words, c.args...)...)
	}
	path, err := serverPath(path)
	if err != nil {
		return "", err
	}
	args := append(words, c.args...)
	args = append(args, "--path="+path)
	if c.read {
		args = append(args, "--skip-plugins", "--skip-themes")
	}
	return sshx.Quote(args...)
}

// serverPath turns "~/x" into "x": the remote shell starts in the home directory (sshd changes to
// it), and a quoted "~" would not be expanded.
func serverPath(path string) (string, error) {
	switch {
	case path == "" || path == "~":
		return "", fmt.Errorf("%q: %w", path, ErrBadPath)
	case strings.HasPrefix(path, "~/"):
		return strings.TrimPrefix(path, "~/"), nil
	case strings.HasPrefix(path, "~"):
		return "", fmt.Errorf("%q (another user's home): %w", path, ErrBadPath)
	}
	return path, nil
}

// trimHome turns "~/x" words into "x": the remote shell starts in the home directory and a quoted
// "~" would not be expanded.
func trimHome(words []string) []string {
	out := make([]string, len(words))
	for i, w := range words {
		out[i] = strings.TrimPrefix(w, "~/")
	}
	return out
}
