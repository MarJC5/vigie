package wpcli

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/MarJC5/vigie/internal/sshx"
)

// CoreVerifyChecksums compares the WordPress files with the checksums wordpress.org publishes for
// the installed version and locale; the server fetches them. It runs before WordPress loads.
func CoreVerifyChecksums() Command {
	return Command{args: []string{"core", "verify-checksums"}, skipPlugins: true, skipThemes: true}
}

// PluginVerifyChecksums compares the files of every plugin wordpress.org knows with its published
// checksums; the others are skipped.
func PluginVerifyChecksums() Command {
	return Command{args: []string{"plugin", "verify-checksums", "--all", "--format=json"}, skipPlugins: true, skipThemes: true}
}

// ErrNotVerified is returned when a checksum command could not compare anything, for example when
// the server could not fetch the checksums: the files are not checked.
var ErrNotVerified = errors.New("files not verified")

// FileChange is how a file differs from the published one.
type FileChange string

const (
	FileChanged FileChange = "changed"
	FileMissing FileChange = "missing"
	FileAdded   FileChange = "added"
)

// FileProblem is a file that differs from the published version: in WordPress, or in a plugin.
type FileProblem struct {
	Plugin string // empty for WordPress's own files
	File   string // relative to WordPress's or the plugin's folder
	Change FileChange
}

// coreWarnings are WP-CLI's warnings about WordPress files, one per file (checksum-command 2.3).
var coreWarnings = []struct {
	prefix string
	change FileChange
}{
	{"Warning: File doesn't exist: ", FileMissing},
	{"Warning: File doesn't verify against checksum: ", FileChanged},
	{"Warning: File should not exist: ", FileAdded},
}

// ParseCoreChecksums reads `core verify-checksums`. Its warnings are the result: an added file
// alone still ends with "Success", so the warnings are read whatever the exit code.
func ParseCoreChecksums(res sshx.Result) ([]FileProblem, error) {
	var problems []FileProblem
	var failure string
	for _, line := range strings.Split(string(res.Stderr), "\n") {
		line = strings.TrimSpace(line)
		for _, w := range coreWarnings {
			if file, ok := strings.CutPrefix(line, w.prefix); ok {
				problems = append(problems, FileProblem{File: file, Change: w.change})
			}
		}
		if msg, ok := strings.CutPrefix(line, "Error: "); ok {
			failure = msg
		}
	}
	switch {
	case res.ExitCode == 0 && strings.Contains(string(res.Stdout), "Success: WordPress installation verifies against checksums."):
		return problems, nil
	case res.ExitCode != 0 && failure == "WordPress installation doesn't verify against checksums.":
		return problems, nil
	case res.ExitCode != 0 && failure != "":
		return nil, fmt.Errorf("%w: %s", ErrNotVerified, failure)
	}
	return nil, fmt.Errorf("core verify-checksums: %w", ErrUnexpectedOutput)
}

// PluginChecksums is the result of `plugin verify-checksums --all`: the files that differ, and how
// many plugins were verified; the skipped ones (unknown to wordpress.org) are not.
type PluginChecksums struct {
	Problems []FileProblem
	Total    int
	Verified int
	Failed   int
	Skipped  []string // their slugs
}

var (
	pluginsSummary = regexp.MustCompile(`(?:Verified|Only verified) (\d+) of (\d+) plugins(?: \(([^)]*)\))?\.`)
	noneVerified   = regexp.MustCompile(`No plugins verified(?: \(([^)]*)\))?\.`)
	summaryCount   = regexp.MustCompile(`(\d+) (failed|skipped)`)
	pluginSkipped  = regexp.MustCompile(`^Warning: Could not retrieve the checksums for version \S+ of plugin (\S+), skipping\.$`)
)

// fileMessages are the plugin command's messages, by file.
var fileMessages = map[string]FileChange{
	"File was added":          FileAdded,
	"Checksum does not match": FileChanged,
}

// ParsePluginChecksums reads `plugin verify-checksums --all --format=json`: the files on stdout
// (only when some differ), the skipped plugins in the warnings, the counts in the last line.
func ParsePluginChecksums(res sshx.Result) (PluginChecksums, error) {
	var out PluginChecksums
	summary := false
	for _, line := range strings.Split(string(res.Stdout)+"\n"+string(res.Stderr), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "["):
			var files []struct {
				Plugin  string `json:"plugin_name"`
				File    string `json:"file"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal([]byte(line), &files); err != nil {
				return PluginChecksums{}, fmt.Errorf("plugin verify-checksums: %w: %w", ErrUnexpectedOutput, err)
			}
			for _, f := range files {
				change, ok := fileMessages[f.Message]
				if !ok {
					change = FileChanged
				}
				out.Problems = append(out.Problems, FileProblem{Plugin: f.Plugin, File: f.File, Change: change})
			}
		case pluginSkipped.MatchString(line):
			out.Skipped = append(out.Skipped, pluginSkipped.FindStringSubmatch(line)[1])
		case pluginsSummary.MatchString(line):
			m := pluginsSummary.FindStringSubmatch(line)
			out.Verified, _ = strconv.Atoi(m[1])
			out.Total, _ = strconv.Atoi(m[2])
			out.Failed, summary = failedOf(m[3]), true
		case noneVerified.MatchString(line):
			m := noneVerified.FindStringSubmatch(line)
			out.Failed, summary = failedOf(m[1]), true
			out.Total = out.Failed + len(out.Skipped)
		}
	}
	if !summary {
		return PluginChecksums{}, fmt.Errorf("plugin verify-checksums: %w", ErrUnexpectedOutput)
	}
	return out, nil
}

// failedOf reads "1 failed, 1 skipped".
func failedOf(counts string) int {
	for _, m := range summaryCount.FindAllStringSubmatch(counts, -1) {
		if m[2] == "failed" {
			n, _ := strconv.Atoi(m[1])
			return n
		}
	}
	return 0
}
