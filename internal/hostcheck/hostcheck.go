// Package hostcheck tests a host before it is added: the connection, PHP on the command line and
// a working WP-CLI, and when there is none, whether Vigie can install it (ADR-0039). It only
// reads; nothing is written on the server.
package hostcheck

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/version"
	"github.com/MarJC5/vigie/internal/wpcli"
	"github.com/MarJC5/vigie/internal/wpcliinstall"
)

var (
	// ErrNoPHP is returned when php does not run on the command line.
	ErrNoPHP = errors.New("PHP not found on the command line")
	// ErrNoWPCLI is returned when no command answered as WP-CLI; Report.Tried lists them.
	ErrNoWPCLI = errors.New("WP-CLI not found")
)

// Usual places, tried after the configured command: the remote shell is not a login shell, so
// PATH may miss what the user sees in an interactive session.
var usual = []string{"wp", "wp-cli", "~/bin/wp", "php ~/wp-cli.phar"}

// Report is the outcome of a connection test.
type Report struct {
	PHPVersion   string
	WPCLI        string   // the command that answered
	WPCLIVersion string   // its version
	Tried        []string // the commands tried, when none answered
	// When none answered: PHP is older than WP-CLI needs, or ~/bin/wp is there but does not answer.
	// Either way Vigie does not offer to install WP-CLI.
	PHPTooOld bool
	Existing  bool
}

// CanInstall tells whether Vigie offers to install WP-CLI: none answered, PHP is recent enough,
// and ~/bin/wp is free.
func (r Report) CanInstall() bool {
	return r.WPCLI == "" && r.PHPVersion != "" && !r.PHPTooOld && !r.Existing
}

// Check runs the test; a connection error (sshx.ConnectionError) stops it at once.
func Check(ctx context.Context, runner sshx.Runner, target sshx.Target, configured string) (Report, error) {
	var report Report
	php, err := sshx.Quote("php", "-r", "echo PHP_VERSION;")
	if err != nil {
		return report, err
	}
	res, err := runner.Run(ctx, target, php)
	if err != nil {
		return report, err
	}
	if res.ExitCode != 0 {
		return report, fmt.Errorf("%w: %s", ErrNoPHP, strings.TrimSpace(string(res.Stderr)))
	}
	report.PHPVersion = string(bytes.TrimSpace(res.Stdout))

	for _, candidate := range candidates(configured) {
		remote, err := wpcli.CLIInfo().Remote(candidate, "")
		if err != nil {
			return report, err
		}
		res, err := runner.Run(ctx, target, remote)
		if err != nil {
			return report, err
		}
		report.Tried = append(report.Tried, candidate)
		if res.ExitCode != 0 {
			continue
		}
		info, err := wpcli.ParseCLIInfo(res.Stdout)
		if err != nil {
			continue
		}
		report.WPCLI, report.WPCLIVersion, report.Tried = candidate, info.WPCLIVersion, nil
		return report, nil
	}
	report.PHPTooOld = version.Compare(report.PHPVersion, wpcliinstall.MinPHP) < 0
	exists, err := sshx.Quote("sh", "-c", `test -e "$HOME/bin/wp"`)
	if err != nil {
		return report, err
	}
	res, err = runner.Run(ctx, target, exists)
	if err != nil {
		return report, err
	}
	report.Existing = res.ExitCode == 0
	return report, ErrNoWPCLI
}

// candidates puts the configured command first, then the usual places, each once.
func candidates(configured string) []string {
	list := []string{}
	if c := strings.TrimSpace(configured); c != "" {
		list = append(list, c)
	}
	for _, u := range usual {
		if len(list) == 0 || list[0] != u {
			list = append(list, u)
		}
	}
	return list
}
