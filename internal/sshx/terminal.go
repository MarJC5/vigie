package sshx

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

// ErrControlCharacter is returned for a folder or key path with a control character (a line
// break, a tab, NUL), which a command copied and pasted could not carry as it is.
var ErrControlCharacter = errors.New("contains a control character")

// ShellCommand returns the ssh command a user can paste in a terminal to open a shell on the host,
// in dir (relative to the home directory, as wpcli.ServerPath gives it, or absolute; "" for the
// home directory). It is written for the user's shell: the arguments in double quotes, the
// command for the server in it, the folder in the server's single quotes (ADR-0023).
func (t Target) ShellCommand(dir string) (string, error) {
	if hasControl(dir) || hasControl(t.KeyPath) {
		return "", ErrControlCharacter
	}
	words := []string{"ssh", "-t"}
	destination := t.Alias
	if destination == "" {
		if t.Port != 0 && t.Port != 22 {
			words = append(words, "-p", strconv.Itoa(t.Port))
		}
		destination = t.User + "@" + t.Hostname
	}
	if t.KeyPath != "" {
		// Only this key, as Vigie connects: some hosts close after a few refused keys.
		words = append(words, "-o", "IdentitiesOnly=yes", "-i", doubleQuote(t.KeyPath))
	}
	words = append(words, destination)
	if dir != "" {
		folder, err := Quote(dir)
		if err != nil {
			return "", err
		}
		words = append(words, doubleQuote("cd "+folder+" && exec $SHELL -l"))
	}
	return strings.Join(words, " "), nil
}

// URL returns the ssh:// address of the host, which the system opens in the user's terminal:
// the alias alone, or the user, host and port. The key and the folder do not fit in it; the
// command of ShellCommand carries them.
func (t Target) URL() string {
	if t.Alias != "" {
		return "ssh://" + t.Alias
	}
	port := t.Port
	if port == 0 {
		port = 22
	}
	return (&url.URL{Scheme: "ssh", User: url.User(t.User), Host: net.JoinHostPort(t.Hostname, strconv.Itoa(port))}).String()
}

// doubleQuote quotes s for a POSIX shell in double quotes: the characters still special there
// ($, `, ", \) escaped.
func doubleQuote(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`, "`", "\\`")
	return `"` + r.Replace(s) + `"`
}

func hasControl(s string) bool {
	return strings.IndexFunc(s, unicode.IsControl) >= 0
}
