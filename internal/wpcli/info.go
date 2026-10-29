package wpcli

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrUnexpectedOutput is returned when WP-CLI printed something else than the expected format.
var ErrUnexpectedOutput = errors.New("unexpected WP-CLI output")

// CLIInfo asks WP-CLI about itself; it loads no WordPress, so it also tells whether a command is
// WP-CLI at all.
func CLIInfo() Command {
	return Command{args: []string{"cli", "info", "--format=json"}, global: true}
}

// Info is what Vigie keeps from `wp cli info`.
type Info struct {
	PHPVersion   string // the PHP that runs WP-CLI on the command line
	WPCLIVersion string
}

// ParseCLIInfo reads the JSON of `wp cli info --format=json` (keys php_version, wp_cli_version).
func ParseCLIInfo(out []byte) (Info, error) {
	var raw struct {
		PHPVersion   string `json:"php_version"`
		WPCLIVersion string `json:"wp_cli_version"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return Info{}, fmt.Errorf("cli info: %w: %v", ErrUnexpectedOutput, err)
	}
	if raw.PHPVersion == "" || raw.WPCLIVersion == "" {
		return Info{}, fmt.Errorf("cli info: %w: missing versions", ErrUnexpectedOutput)
	}
	return Info{PHPVersion: raw.PHPVersion, WPCLIVersion: raw.WPCLIVersion}, nil
}
