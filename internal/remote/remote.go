// Package remote builds the shell commands Vigie runs on a server around WP-CLI: during an update,
// backups, restores, the size and the new lines of debug.log, the dump tool; during a scan, the PHP
// files of the uploads and what debug.log holds. They are fixed; their values are quoted, and a path
// outside Vigie's backups or a site's folders is refused.
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
	return ".vigie/backups/" + site.String() + "/" + at.UTC().Format(BackupTimeLayout)
}

// BackupTimeLayout names the folder of a run's backups after its time, in UTC.
const BackupTimeLayout = "20060102-150405"

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

// DebugLog reads, in one go, a scan's view of a site's debug.log (ADR-0028): nothing when there is no
// such file; else its size, then the last fatal and parse errors of its last megabyte, at most
// debugLines of them.
func DebugLog(file string) (string, error) {
	if err := folder(file); err != nil {
		return "", err
	}
	q, err := quoted(file)
	if err != nil {
		return "", err
	}
	return "f=" + q + `; [ -f "$f" ] || exit 0; wc -c < "$f"; tail -c ` + strconv.Itoa(debugTail) + ` "$f" | ` + grepText + ` -E 'PHP (Fatal|Parse) error' | tail -n ` +
		strconv.Itoa(debugLines), nil
}

// grepText is grep for a log: in the C locale and reading every byte as text, so that no line is
// left out for its bytes (GNU grep drops, in a UTF-8 locale, a line that is not UTF-8).
const grepText = "LC_ALL=C grep -a"

const (
	debugTail  = 1 << 20 // the bytes of the end of debug.log searched
	debugLines = 50      // the error lines transferred at most
	// RecentFatals is how far back a fatal error counts as recent.
	RecentFatals = 7 * 24 * time.Hour
	// fatalShown is how much of the latest error's message is kept.
	fatalShown = 300
)

// DebugLogRead is what a scan read of a site's debug.log.
type DebugLogRead struct {
	Exists  bool
	Size    int64
	Recent  int       // the fatal and parse errors of the last RecentFatals
	Last    time.Time // the latest of them, UTC
	Message string    // its message, without its time, cut at fatalShown characters
}

// logTime is the time PHP writes before each line of its log ("[d-M-Y H:i:s e]"), in UTC since
// WordPress sets it.
var logTime = regexp.MustCompile(`^\[(\d{2}-[A-Z][a-z]{2}-\d{4} \d{2}:\d{2}:\d{2}) ([^\]]+)\] `)

// ParseDebugLog reads DebugLog's answer; the errors without a time PHP wrote are not counted.
func ParseDebugLog(out []byte, now time.Time) (DebugLogRead, error) {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return DebugLogRead{}, nil
	}
	size, err := strconv.ParseInt(strings.TrimSpace(lines[0]), 10, 64)
	if err != nil || size < 0 {
		return DebugLogRead{}, fmt.Errorf("size %q: %w", lines[0], wpcli.ErrUnexpectedOutput)
	}
	read := DebugLogRead{Exists: true, Size: size}
	for _, line := range lines[1:] {
		m := logTime.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		loc, err := time.LoadLocation(m[2])
		if err != nil {
			continue
		}
		at, err := time.ParseInLocation("02-Jan-2006 15:04:05", m[1], loc)
		if err != nil || now.Sub(at) > RecentFatals {
			continue
		}
		read.Recent++
		if at = at.UTC(); !at.Before(read.Last) {
			read.Last = at
			message := []rune(strings.TrimSpace(line[len(m[0]):]))
			read.Message = string(message[:min(len(message), fatalShown)])
		}
	}
	return read, nil
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

// literal is a string that PHP reads as it is written: the same quote at both ends, and inside it
// no `$`, `\`, `{` nor `}`, with which a string between double quotes runs code ("{${f()}}").
const literal = `('[^'$\\{}]*'|"[^"$\\{}]*")`

// status and code are the literals of a 403 or 404 status: the status line, or the code after the
// server's protocol.
const (
	status = `('HTTP/1\.[01]\s+40[34][^'$\\{}]*'|"HTTP/1\.[01]\s+40[34][^"$\\{}]*")`
	code   = `('\s*40[34][^'$\\{}]*'|"\s*40[34][^"$\\{}]*")`
)

var (
	// silence is the empty index.php WordPress and plugins put in folders so they cannot be listed:
	// "<?php // Silence is golden." and its variants, nothing else.
	silence = regexp.MustCompile(`(?i)^<\?php\s*(//|#|/\*)?\s*silence is golden\.?\s*(\*/)?\s*(\?>)?\s*$`)
	// forbidden is an index.php that only answers 403 or 404, nothing else: the status line written
	// out, or the server's protocol followed by the code, then perhaps an exit with a literal.
	forbidden = regexp.MustCompile(`(?i)^<\?php\s+header\s*\(\s*(` + status + `|\$_SERVER\[\s*('SERVER_PROTOCOL'|"SERVER_PROTOCOL")\s*\]\s*\.\s*` + code + `)\s*\)\s*;\s*((exit|die)\s*(\(\s*` + literal + `?\s*\))?\s*;\s*)?(\?>)?\s*$`)
	// stops is a file whose first statement stops PHP: nothing after it runs, as in Matomo's
	// config.ini.php ("; <?php exit; ?> DO NOT REMOVE THIS LINE"). Its message is a literal: one
	// that runs code before PHP stops is not dropped.
	stops = regexp.MustCompile(`(?i)^;?\s*<\?php\s+(exit|die)\s*(\(\s*` + literal + `?\s*\))?\s*;`)
)

// dropped tells whether a file found in the uploads was put there on purpose and runs nothing: a file
// whose first statement stops PHP, or a whole index.php that only stops folder listings, is empty or
// holds no PHP at all (Matomo writes "//hello" in some).
func dropped(path, size, content string) bool {
	content = strings.TrimSpace(content)
	if stops.MatchString(content) {
		return true
	}
	// The whole file must have been read, or code could follow the part that looks harmless.
	n, err := strconv.Atoi(size)
	whole := err == nil && n <= stubBytes && strings.HasSuffix(path, "/index.php")
	return whole && (!strings.Contains(content, "<?") || silence.MatchString(content) || forbidden.MatchString(content))
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

// RemoveSiteBackups removes every backup of the sites given: their folders under ~/.vigie/backups,
// when a site leaves Vigie (ADR-0033).
func RemoveSiteBackups(sites ...uuid.UUID) (string, error) {
	if len(sites) == 0 {
		return "", fmt.Errorf("no site: %w", ErrBadPath)
	}
	dirs := make([]string, len(sites))
	for i, site := range sites {
		dirs[i] = ".vigie/backups/" + site.String()
	}
	return quotedLine("rm -rf", dirs...)
}

// BackupSizes lists the runs of a site's backups with their sizes in kilobytes, for a scan: it only
// reads, and answers nothing, without failing, when the site has none (ADR-0033).
func BackupSizes(site uuid.UUID) (string, error) {
	q, err := quoted(".vigie/backups/" + site.String())
	if err != nil {
		return "", err
	}
	return "cd " + q + " 2>/dev/null && du -sk -- */ 2>/dev/null; true", nil
}

// BackupSize is one run of a site's backups and its size on disk.
type BackupSize struct {
	Run   string // the run's folder, its time (BackupTimeLayout)
	Bytes int64
}

// ParseBackupSizes reads BackupSizes's answer, oldest first; a folder that is not a run is left
// out. Nothing answered is an empty list: read, and none there.
func ParseBackupSizes(out []byte) []BackupSize {
	sizes := []BackupSize{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		run := strings.TrimSuffix(fields[1], "/")
		kb, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil || !backupTime.MatchString(run) {
			continue
		}
		sizes = append(sizes, BackupSize{Run: run, Bytes: kb * 1024})
	}
	slices.SortFunc(sizes, func(a, b BackupSize) int { return strings.Compare(a.Run, b.Run) })
	return sizes
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
