package sshx

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ErrConfigDepth is returned for includes nested deeper than OpenSSH reads them.
var ErrConfigDepth = errors.New("too many nested includes")

// maxConfigDepth is OpenSSH's READCONF_MAX_DEPTH (readconf.c).
const maxConfigDepth = 16

// ConfigNames lists the names of the Host lines of the user's ~/.ssh/config and of the files it
// includes, in the order they come, each once, and counts the patterns left out (with "*" or "?",
// or negated with "!"): they name no one host. What each name stands for is ssh's to tell
// (ssh -G), so Match blocks and the other keywords are not read. A missing ~/.ssh/config lists
// nothing. Vigie never writes these files.
func ConfigNames(home string) (names []string, patterns int, err error) {
	r := configReader{home: home, seen: map[string]bool{}}
	if err := r.read(filepath.Join(home, ".ssh", "config"), 0); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, 0, err
	}
	return r.names, r.patterns, nil
}

type configReader struct {
	home     string
	names    []string
	patterns int
	seen     map[string]bool
}

func (r *configReader) read(file string, depth int) error {
	if depth > maxConfigDepth {
		return fmt.Errorf("%s: %w", file, ErrConfigDepth)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	for line := range strings.Lines(string(data)) {
		keyword, args := configLine(line)
		switch strings.ToLower(keyword) {
		case "host":
			for _, name := range args {
				if strings.ContainsAny(name, "*?") || strings.HasPrefix(name, "!") {
					r.patterns++
					continue
				}
				if r.seen[name] {
					continue
				}
				r.seen[name] = true
				r.names = append(r.names, name)
			}
		case "include":
			for _, pattern := range args {
				if err := r.include(pattern, depth); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// include reads the files a pattern matches, in lexical order; a pattern that matches none is
// skipped, as ssh does. Relative paths are under ~/.ssh, "~" is the user's home.
func (r *configReader) include(pattern string, depth int) error {
	switch {
	case pattern == "~" || strings.HasPrefix(pattern, "~/"):
		pattern = filepath.Join(r.home, pattern[1:])
	case !filepath.IsAbs(pattern):
		pattern = filepath.Join(r.home, ".ssh", pattern)
	}
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("include %q: %w", pattern, err)
	}
	slices.Sort(files)
	for _, f := range files {
		if info, err := os.Stat(f); err != nil || info.IsDir() {
			continue
		}
		if err := r.read(f, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// configLine splits a line of ssh_config: the keyword, then its arguments, separated by
// whitespace or one "=", double quotes keeping spaces in an argument, "#" outside quotes starting
// a comment.
func configLine(line string) (string, []string) {
	var fields []string
	var field strings.Builder
	quoted, inField, equals := false, false, false
	end := func() {
		if inField {
			fields = append(fields, field.String())
			field.Reset()
			inField = false
		}
	}
	for _, c := range strings.TrimSpace(line) {
		switch {
		case c == '"':
			quoted, inField = !quoted, true
		case quoted:
			field.WriteRune(c)
		case c == '#':
			end()
			return split(fields)
		case c == '=' && !equals && (len(fields) == 0 || (len(fields) == 1 && !inField)):
			equals = true
			end()
		case c == ' ' || c == '\t':
			end()
		default:
			field.WriteRune(c)
			inField = true
		}
	}
	end()
	return split(fields)
}

func split(fields []string) (string, []string) {
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], fields[1:]
}
