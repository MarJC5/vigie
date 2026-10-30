// Package remote builds the shell commands Vigie runs on a server around WP-CLI during an update:
// backups, restores, the size and the new lines of debug.log, the dump tool. They are fixed; their
// values are quoted, and a path outside Vigie's backups or a site's folders is refused.
package remote

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// ErrBadPath is returned for a path a command must not touch.
var ErrBadPath = errors.New("path refused")

// The backups of one run: .vigie/backups/<site ID>/<UTC time>, relative to the home directory.
var (
	backupDir  = regexp.MustCompile(`^\.vigie/backups/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/[0-9]{8}-[0-9]{6}$`)
	fileName   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	backupTime = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}$`)
)

// BackupDir is the folder of the backups of a run started at at.
func BackupDir(site uuid.UUID, at time.Time) string {
	return ".vigie/backups/" + site.String() + "/" + at.UTC().Format("20060102-150405")
}

// backupFile checks a file of a run's backups with the extension given.
func backupFile(file, ext string) error {
	dir, name := path.Split(file)
	if !backupDir.MatchString(strings.TrimSuffix(dir, "/")) || !fileName.MatchString(name) || !strings.HasSuffix(name, ext) {
		return fmt.Errorf("%q: %w", file, ErrBadPath)
	}
	return nil
}

// folder checks an absolute, clean folder other than the root of the file system.
func folder(dir string) error {
	if !strings.HasPrefix(dir, "/") || path.Clean(dir) != dir || dir == "/" || strings.ContainsAny(dir, "\n") {
		return fmt.Errorf("%q: %w", dir, ErrBadPath)
	}
	return nil
}

func quoted(args ...string) (string, error) {
	return sshx.Quote(args...)
}

// RealRoot answers the site's folder with its links resolved, as WordPress writes its paths.
func RealRoot(root string) (string, error) {
	p, err := wpcli.ServerPath(root)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrBadPath, err)
	}
	q, err := quoted(p)
	if err != nil {
		return "", err
	}
	return "cd " + q + " && pwd -P", nil
}

// DumpTool answers the path of the database dump tool WP-CLI uses, exit 1 when there is none.
func DumpTool() string {
	return "command -v mariadb-dump || command -v mysqldump"
}

// MakeDir creates a run's backups folder, readable by the account only.
func MakeDir(dir string) (string, error) {
	if !backupDir.MatchString(dir) {
		return "", fmt.Errorf("%q: %w", dir, ErrBadPath)
	}
	q, err := quoted(dir)
	return "umask 077 && mkdir -p " + q, err
}

// Compress compresses a database dump of the backups.
func Compress(file string) (string, error) {
	if err := backupFile(file, ".sql"); err != nil {
		return "", err
	}
	q, err := quoted(file)
	return "gzip -f " + q, err
}

// Uncompress writes the dump next to its compressed copy, which stays.
func Uncompress(file string) (string, error) {
	if err := backupFile(file, ".sql.gz"); err != nil {
		return "", err
	}
	q, err := quoted(file)
	return "gzip -dkf " + q, err
}

// RemoveFile removes a file of the backups, for example a dump once restored.
func RemoveFile(file string) (string, error) {
	if err := backupFile(file, ""); err != nil {
		return "", err
	}
	q, err := quoted(file)
	return "rm -f " + q, err
}

// Archive stores a folder (a plugin or theme) in a file of the backups.
func Archive(out, dir string) (string, error) {
	if err := backupFile(out, ".tar.gz"); err != nil {
		return "", err
	}
	if err := folder(dir); err != nil {
		return "", err
	}
	return quotedLine("tar -czf", out, "-C", path.Dir(dir), path.Base(dir))
}

// ArchiveCore stores the files of WordPress: the site's folder without its content, its
// configuration (which holds the database password) and the maintenance file.
func ArchiveCore(out, root string) (string, error) {
	if err := backupFile(out, ".tar.gz"); err != nil {
		return "", err
	}
	if err := folder(root); err != nil {
		return "", err
	}
	return quotedLine("tar -czf", out, "-C", root, "--exclude=./wp-content", "--exclude=./wp-config.php", "--exclude=./.maintenance", ".")
}

// Restore puts a folder back as archived: the folder is removed first, so no new file stays.
func Restore(archive, dir string) (string, error) {
	if err := backupFile(archive, ".tar.gz"); err != nil {
		return "", err
	}
	if err := folder(dir); err != nil {
		return "", err
	}
	rm, err := quotedLine("rm -rf", dir)
	if err != nil {
		return "", err
	}
	tar, err := quotedLine("tar -xzf", archive, "-C", path.Dir(dir))
	return rm + " && " + tar, err
}

// RestoreCore puts the files of WordPress back over the site's folder.
func RestoreCore(archive, root string) (string, error) {
	if err := backupFile(archive, ".tar.gz"); err != nil {
		return "", err
	}
	if err := folder(root); err != nil {
		return "", err
	}
	return quotedLine("tar -xzf", archive, "-C", root)
}

// FileSize answers the size of a file in bytes, for example debug.log before an update.
func FileSize(file string) (string, error) {
	if err := folder(file); err != nil {
		return "", err
	}
	return quotedLine("wc -c <", file)
}

// FatalsSince answers the first fatal errors written to a log after offset bytes.
func FatalsSince(file string, offset int64) (string, error) {
	if err := folder(file); err != nil {
		return "", err
	}
	if offset < 0 {
		return "", fmt.Errorf("offset %d: %w", offset, ErrBadPath)
	}
	q, err := quoted(file)
	return "tail -c +" + strconv.FormatInt(offset+1, 10) + " " + q + " | grep -m 3 'PHP Fatal error'", err
}

// UploadsPHP lists the PHP files under a site's uploads folder, where WordPress never puts any: one
// line per file, at most uploadsShown and one more to tell there are more. Each comes with its size
// and the start of its content after tabs (the first stubBytes of an index.php, the first line of
// another file), so that the files plugins drop on purpose are told apart: the empty index.php that
// stops folder listings, and files that stop PHP at once. Nothing more of a file is read. A site
// without the folder lists nothing.
func UploadsPHP(root string) (string, error) {
	if err := folder(root); err != nil {
		return "", err
	}
	q, err := quoted(root + "/wp-content/uploads")
	if err != nil {
		return "", err
	}
	n := strconv.Itoa(stubBytes)
	return "find " + q + ` -type f \( -iname '*.php' -o -iname '*.php[0-9]' -o -iname '*.phtml' -o -iname '*.pht' -o -iname '*.phar' \) 2>/dev/null | head -n ` +
		strconv.Itoa(UploadsShown+1) +
		` | while IFS= read -r f; do s=$(wc -c < "$f" | tr -d ' '); case "$f" in */index.php) c=$(head -c ` + n + ` "$f");; *) c=$(head -n 1 "$f" | head -c ` + n +
		`);; esac; printf '%s\t%s\t%s\n' "$f" "$s" "$(printf '%s' "$c" | tr '\t\r\n' '   ')"; done`, nil
}

// UploadsShown is how many PHP files in uploads are listed.
const UploadsShown = 50

// stubBytes is how much of a file is read: more than any file dropped on purpose.
const stubBytes = 120

var (
	// silence is the empty index.php WordPress and plugins put in folders so they cannot be listed:
	// "<?php // Silence is golden." and its variants, nothing else.
	silence = regexp.MustCompile(`(?i)^<\?php\s*(//|#|/\*)?\s*silence is golden\.?\s*(\*/)?\s*(\?>)?\s*$`)
	// forbidden is an index.php that only answers 403 or 404, nothing else.
	forbidden = regexp.MustCompile(`(?i)^<\?php\s+header\s*\(\s*['"]HTTP/1\.[01]\s+40[34][^'"]*['"]\s*\)\s*;\s*((exit|die)\s*(\(\s*\))?\s*;\s*)?(\?>)?\s*$`)
	// stops is a file whose first statement stops PHP: nothing after it runs, as in Matomo's
	// config.ini.php ("; <?php exit; ?> DO NOT REMOVE THIS LINE").
	stops = regexp.MustCompile(`(?i)^;?\s*<\?php\s+(exit|die)\s*(\(\s*(['"][^'"]*['"])?\s*\))?\s*;`)
)

// dropped tells whether a file found in the uploads was put there on purpose and runs nothing: a file
// whose first statement stops PHP, or a whole index.php that only stops folder listings.
func dropped(path, size, content string) bool {
	content = strings.TrimSpace(content)
	if stops.MatchString(content) {
		return true
	}
	// The whole file must have been read, or code could follow the part that looks harmless.
	n, err := strconv.Atoi(size)
	whole := err == nil && n <= stubBytes && strings.HasSuffix(path, "/index.php")
	return whole && (silence.MatchString(content) || forbidden.MatchString(content))
}

// ParseUploadsPHP reads UploadsPHP's answer: the files relative to the uploads folder, without
// those put there on purpose that run nothing, and whether there are more than UploadsShown.
func ParseUploadsPHP(root string, out []byte) (files []string, more bool) {
	prefix := root + "/wp-content/uploads/"
	lines := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if lines++; lines > UploadsShown {
			return files, true
		}
		fields := strings.SplitN(line, "\t", 3)
		path := strings.TrimSpace(fields[0])
		if len(fields) == 3 && dropped(path, fields[1], fields[2]) {
			continue
		}
		files = append(files, strings.TrimPrefix(path, prefix))
	}
	return files, false
}

// ListBackups lists the runs of a site that have backups.
func ListBackups(site uuid.UUID) (string, error) {
	return quotedLine("ls -1", ".vigie/backups/"+site.String())
}

// RemoveBackup removes the backups of one run.
func RemoveBackup(dir string) (string, error) {
	if !backupDir.MatchString(dir) {
		return "", fmt.Errorf("%q: %w", dir, ErrBadPath)
	}
	return quotedLine("rm -rf", dir)
}

// quotedLine is a fixed command followed by quoted arguments.
func quotedLine(command string, args ...string) (string, error) {
	q, err := quoted(args...)
	if err != nil {
		return "", err
	}
	return command + " " + q, nil
}

// ParseSize reads FileSize's answer; a file that does not exist yet is empty.
func ParseSize(res sshx.Result) (int64, error) {
	if res.ExitCode != 0 {
		if strings.Contains(string(res.Stderr), "No such file") {
			return 0, nil
		}
		return 0, wpcli.Done(res)
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(res.Stdout)), 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("size %q: %w", res.Stdout, wpcli.ErrUnexpectedOutput)
	}
	return n, nil
}

// ParseBackups reads ListBackups's answer: the runs' times, oldest first; other names are ignored.
func ParseBackups(out []byte) []string {
	var times []string
	for _, line := range strings.Split(string(out), "\n") {
		if backupTime.MatchString(line) {
			times = append(times, line)
		}
	}
	slices.Sort(times)
	return times
}

// ParseFatals reads FatalsSince's answer: the lines found, none when grep found nothing.
func ParseFatals(res sshx.Result) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(string(res.Stdout)), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
