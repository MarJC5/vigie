package remote_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/MarJC5/vigie/internal/errorlog"
	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/sshx"
)

var site = uuid.MustParse("01a18704-356b-77be-ab41-b92a0d67a6bc")

func TestBackupDir(t *testing.T) {
	at := time.Date(2026, 10, 29, 14, 12, 3, 0, time.FixedZone("CET", 3600))
	if got := remote.BackupDir(site, at); got != ".vigie/backups/01a18704-356b-77be-ab41-b92a0d67a6bc/20261029-131203" {
		t.Errorf("remote.BackupDir() = %q, in UTC", got)
	}
}

func TestRemoteCommands(t *testing.T) {
	dir := ".vigie/backups/01a18704-356b-77be-ab41-b92a0d67a6bc/20261029-131203"
	tests := []struct {
		name string
		got  func() (string, error)
		want string
	}{
		{"root", func() (string, error) { return remote.RealRoot("~/sites/a b") }, `cd 'sites/a b' && pwd -P`},
		{"absolute root", func() (string, error) { return remote.RealRoot("/var/www/site") }, `cd '/var/www/site' && pwd -P`},
		{"folder", func() (string, error) { return remote.MakeDir(dir) }, `umask 077 && mkdir -p '` + dir + `'`},
		{"compress", func() (string, error) { return remote.Compress(dir + "/database.sql") }, `gzip -f '` + dir + `/database.sql'`},
		{"uncompress", func() (string, error) { return remote.Uncompress(dir + "/database.sql.gz") }, `gzip -dkf '` + dir + `/database.sql.gz'`},
		{"remove a dump", func() (string, error) { return remote.RemoveFile(dir + "/database.sql") }, `rm -f '` + dir + `/database.sql'`},
		{"archive", func() (string, error) {
			return remote.Archive(dir+"/plugin-akismet.tar.gz", "/home/u/sites/a/wp-content/plugins/akismet")
		},
			`tar -czf '` + dir + `/plugin-akismet.tar.gz' '-C' '/home/u/sites/a/wp-content/plugins' 'akismet'`},
		{"archive core", func() (string, error) { return remote.ArchiveCore(dir+"/core.tar.gz", "/home/u/sites/a") },
			`tar -czf '` + dir + `/core.tar.gz' '-C' '/home/u/sites/a' '--exclude=./wp-content' '--exclude=./wp-config.php' '--exclude=./.maintenance' '.'`},
		{"restore", func() (string, error) {
			return remote.Restore(dir+"/plugin-akismet.tar.gz", "/home/u/sites/a/wp-content/plugins/akismet")
		},
			`rm -rf '/home/u/sites/a/wp-content/plugins/akismet' && tar -xzf '` + dir + `/plugin-akismet.tar.gz' '-C' '/home/u/sites/a/wp-content/plugins'`},
		{"restore core", func() (string, error) { return remote.RestoreCore(dir+"/core.tar.gz", "/home/u/sites/a") },
			`tar -xzf '` + dir + `/core.tar.gz' '-C' '/home/u/sites/a'`},
		{"size", func() (string, error) { return remote.FileSize("/home/u/sites/a/wp-content/debug.log") }, `wc -c < '/home/u/sites/a/wp-content/debug.log'`},
		{"fatal errors", func() (string, error) { return remote.FatalsSince("/home/u/sites/a/wp-content/debug.log", 1200) },
			`tail -c +1201 '/home/u/sites/a/wp-content/debug.log' | LC_ALL=C grep -a -m 3 'PHP Fatal error'`},
		{"debug.log of a scan", func() (string, error) { return remote.DebugLog("/home/u/sites/a/wp-content/debug.log") },
			`f='/home/u/sites/a/wp-content/debug.log'; [ -f "$f" ] || exit 0; wc -c < "$f"; tail -c 1048576 "$f" | LC_ALL=C grep -a -E 'PHP (Fatal|Parse) error' | tail -n 50`},
		{"a site's errors", func() (string, error) { return remote.ErrorLines("/home/u/sites/a/wp-content/debug.log") },
			`f='/home/u/sites/a/wp-content/debug.log'; [ -f "$f" ] || exit 0; [ -r "$f" ] || { echo "cannot read $f" >&2; exit 1; }; ` +
				`s=$(wc -c < "$f" | tr -d ' '); echo "$s"; ` +
				`if [ "$s" -gt 1048576 ]; then tail -c 1048576 "$f" | tail -n +2; else cat "$f"; fi | ` +
				`LC_ALL=C grep -a -E '^\[[^]]+\] PHP (Fatal error|Parse error|Recoverable fatal error|Warning):|^  thrown in .* on line [0-9]+$' | ` +
				`cut -b 1-2000 | tail -n 501`},
		{"backups", func() (string, error) { return remote.ListBackups(site) }, `ls -1 '.vigie/backups/01a18704-356b-77be-ab41-b92a0d67a6bc'`},
		{"remove a backup", func() (string, error) { return remote.RemoveBackup(dir) }, `rm -rf '` + dir + `'`},
		{"backups of sites leaving", func() (string, error) { return remote.RemoveSiteBackups(site, site) },
			`rm -rf '.vigie/backups/01a18704-356b-77be-ab41-b92a0d67a6bc' '.vigie/backups/01a18704-356b-77be-ab41-b92a0d67a6bc'`},
		{"backups of a scan", func() (string, error) { return remote.BackupSizes(site) },
			`cd '.vigie/backups/01a18704-356b-77be-ab41-b92a0d67a6bc' 2>/dev/null && du -sk -- */ 2>/dev/null; true`},
	}
	for _, tt := range tests {
		got, err := tt.got()
		if err != nil || got != tt.want {
			t.Errorf("%s:\n got %s, %v\nwant %s", tt.name, got, err, tt.want)
		}
	}
	if remote.DumpTool() != "command -v mariadb-dump || command -v mysqldump" {
		t.Errorf("remote.DumpTool() = %q", remote.DumpTool())
	}
}

// Nothing outside the backups or the site's folders: a bad path never reaches the server.
func TestRemoteCommandsRefuseBadPaths(t *testing.T) {
	for name, got := range map[string]func() (string, error){
		"backup outside":        func() (string, error) { return remote.MakeDir(".vigie/other") },
		"backup going up":       func() (string, error) { return remote.RemoveBackup(".vigie/backups/" + site.String() + "/..") },
		"backup of no time":     func() (string, error) { return remote.RemoveBackup(".vigie/backups/" + site.String() + "/latest") },
		"archive into the site": func() (string, error) { return remote.Archive("/tmp/a.tar.gz", "/home/u/sites/a/wp-content/plugins/x") },
		"relative folder": func() (string, error) {
			return remote.Archive(".vigie/backups/"+site.String()+"/20261029-131203/x.tar.gz", "plugins/x")
		},
		"folder going up": func() (string, error) {
			return remote.Restore(".vigie/backups/"+site.String()+"/20261029-131203/x.tar.gz", "/home/u/../x")
		},
		"the root itself": func() (string, error) {
			return remote.Restore(".vigie/backups/"+site.String()+"/20261029-131203/x.tar.gz", "/")
		},
		"debug log not absolute":   func() (string, error) { return remote.FileSize("wp-content/debug.log") },
		"a negative offset":        func() (string, error) { return remote.FatalsSince("/a/debug.log", -1) },
		"a scan's log relative":    func() (string, error) { return remote.DebugLog("debug.log") },
		"errors of a relative log": func() (string, error) { return remote.ErrorLines("wp-content/debug.log") },
		"a dump out of a backup":   func() (string, error) { return remote.Uncompress("/tmp/x.sql.gz") },
		"home as a root":           func() (string, error) { return remote.RealRoot("~") },
	} {
		if _, err := got(); !errors.Is(err, remote.ErrBadPath) {
			t.Errorf("%s: %v, want remote.ErrBadPath", name, err)
		}
	}
}

func TestParsers(t *testing.T) {
	if n, err := remote.ParseSize(sshx.Result{Stdout: []byte("1234\n")}); err != nil || n != 1234 {
		t.Errorf("ParseSize = %d, %v", n, err)
	}
	// No debug.log yet: nothing was written, the size is 0.
	missing := sshx.Result{ExitCode: 1, Stderr: []byte("sh: 1: cannot open /a/debug.log: No such file\n")}
	if n, err := remote.ParseSize(missing); err != nil || n != 0 {
		t.Errorf("ParseSize(missing) = %d, %v", n, err)
	}
	backups := remote.ParseBackups([]byte("20261029-131203\nnotes.txt\n20261028-090000\n20261029-140000\n"))
	if want := []string{"20261028-090000", "20261029-131203", "20261029-140000"}; !slices.Equal(backups, want) {
		t.Errorf("ParseBackups = %v, want %v (only times, sorted)", backups, want)
	}
	// The runs with their sizes, oldest first; a folder that is not a run is left out (ADR-0033).
	sizes := remote.ParseBackupSizes([]byte("47104\t20261029-131203/\n12\tnotes/\n44032\t20261028-090000/\n"))
	if want := []remote.BackupSize{{Run: "20261028-090000", Bytes: 44032 * 1024}, {Run: "20261029-131203", Bytes: 47104 * 1024}}; !slices.Equal(sizes, want) {
		t.Errorf("ParseBackupSizes = %v, want %v", sizes, want)
	}
	if sizes := remote.ParseBackupSizes(nil); sizes == nil || len(sizes) != 0 {
		t.Errorf("ParseBackupSizes(nothing) = %#v, want an empty list: read, none there", sizes)
	}
	fatal := "[29-Oct-2026 14:12:18 UTC] PHP Fatal error:  Uncaught Error: Call to undefined function x() in /a/b.php:6"
	if got := remote.ParseFatals(sshx.Result{Stdout: []byte(fatal + "\n")}); !slices.Equal(got, []string{fatal}) {
		t.Errorf("ParseFatals = %q", got)
	}
	// grep finds nothing: exit 1, no line.
	if got := remote.ParseFatals(sshx.Result{ExitCode: 1}); len(got) != 0 {
		t.Errorf("ParseFatals(none) = %q", got)
	}
}

func TestUploadsPHP(t *testing.T) {
	got, err := remote.UploadsPHP("/home/u/sites/a")
	// Each file with its size and the start of its content: a whole small index.php, the first line
	// of another file, never more than 120 bytes.
	for _, part := range []string{"find '/home/u/sites/a/wp-content/uploads' -type f", "| head -n 51 |", "wc -c < \"$f\"",
		"*/index.php) c=$(head -c 120 \"$f\")", "*) c=$(head -n 1 \"$f\" | head -c 120)"} {
		if err != nil || !strings.Contains(got, part) {
			t.Errorf("UploadsPHP() = %s, %v\nwant it with %s", got, err, part)
		}
	}
	if _, err := remote.UploadsPHP("sites/a"); !errors.Is(err, remote.ErrBadPath) {
		t.Errorf("UploadsPHP(relative) = %v, want ErrBadPath", err)
	}
}

func recordedResult(t *testing.T, name string) (string, sshx.Result) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "recorded", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var r struct {
		Args           []string
		Exit           int
		Stdout, Stderr string
	}
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	return r.Args[0], sshx.Result{ExitCode: r.Exit, Stdout: []byte(r.Stdout), Stderr: []byte(r.Stderr)}
}

func TestParseUploadsPHP(t *testing.T) {
	const root = "/home/vigie/sites/security"
	line, res := recordedResult(t, "uploads-php")
	if want, _ := remote.UploadsPHP(root); line != want {
		t.Fatalf("recorded %s, Vigie runs %s: run make record", line, want)
	}
	// Dropped on purpose and running nothing: Solid Security's "Silence is golden", Matomo's
	// config.ini.php that stops PHP on its first line. An index.php with anything more is listed.
	files, more := remote.ParseUploadsPHP(root, res.Stdout)
	slices.Sort(files)
	if !reflect.DeepEqual(files, []string{"2026/10/fixture-shell.php", "fixture-old.PHTML", "ithemes-security/backups/index.php"}) || more {
		t.Errorf("ParseUploadsPHP() = %v, %v", files, more)
	}
	_, clean := recordedResult(t, "uploads-php-clean")
	if files, more := remote.ParseUploadsPHP("/home/vigie/sites/plain", clean.Stdout); len(files) != 0 || more {
		t.Errorf("ParseUploadsPHP(clean) = %v, %v", files, more)
	}
	// One more than shown: there are more.
	var many strings.Builder
	for i := range remote.UploadsShown + 1 {
		fmt.Fprintf(&many, "%s/wp-content/uploads/f%d.php\t9\t<?php x\n", root, i)
	}
	if files, more := remote.ParseUploadsPHP(root, []byte(many.String())); len(files) != remote.UploadsShown || !more {
		t.Errorf("ParseUploadsPHP(51 files) = %d files, more %v", len(files), more)
	}
	for _, tc := range []struct {
		file, size, content string
		dropped             bool
	}{
		{"a/index.php", "28", "<?php // Silence is golden.", true},
		{"a/index.php", "30", "<?php  # silence is golden ?>", true},
		{"a/index.php", "30", "<?php /* Silence is golden */", true},
		{"a/index.php", "45", "<?php header('HTTP/1.0 403 Forbidden'); exit; ?>", true},
		{"m/config/config.ini.php", "900", "; <?php exit; ?> DO NOT REMOVE THIS LINE", true},
		{"a/cache.php", "500", "<?php die(); ?>", true},
		{"a/index.php", "40", "<?php // Silence is golden. phpinfo();", false},
		{"a/index.php", "40", "<?php phpinfo(); // Silence is golden", false},
		// Looks harmless, but longer than what was read: code may follow.
		{"a/index.php", "5000", "<?php // Silence is golden.", false},
		// Only an index.php is read whole.
		{"a/other.php", "28", "<?php // Silence is golden.", false},
		{"a/x.php", "12", "<?php exit_handler(); phpinfo();", false},
		// A whole index.php without PHP runs nothing: empty, or Matomo's "//hello".
		{"a/index.php", "0", "", true},
		{"a/index.php", "7", "//hello", true},
		{"a/index.php", "86", `<?php  header($_SERVER['SERVER_PROTOCOL'] . ' 403 Forbidden' ); die( '403 Forbidden' ); `, true},
		{"a/index.php", "43", `<?php header('HTTP/1.1 403 Forbidden'); exit;`, true},
		{"a/index.php", "16", "<?php phpinfo();", false},
		{"a/index.php", "16", "<?= phpinfo() ?>", false},
		{"a/index.php", "24", "//hello <?php phpinfo();", false},
		{"a/index.php", "40", `<?php header('HTTP/1.1 403'); phpinfo();`, false},
		// A string between double quotes runs what "{${...}}" holds: not a literal.
		{"a/index.php", "45", `<?php header("HTTP/1.1 403 {${phpinfo()}}");`, false},
		{"a/index.php", "30", `<?php exit("{${phpinfo()}}");`, false},
		{"a/x.php", "29", `<?php die("{${phpinfo()}}");`, false},
		// Quotes that do not match: PHP reads one string up to the same quote, then what follows.
		{"a/x.php", "38", `<?php die("a') ; {${phpinfo()}}");`, false},
		{"a/index.php", "62", `<?php header("HTTP/1.1 403'); exit('"); phpinfo(); //');`, false},
		// Only a whole index.php may hold no PHP.
		{"a/x.php", "0", "", false},
		{"a/x.php", "7", "//hello", false},
		{"a/index.php", "5000", strings.Repeat("/", 120), false},
	} {
		line := root + "/wp-content/uploads/" + tc.file + "\t" + tc.size + "\t" + tc.content + "\n"
		if files, _ := remote.ParseUploadsPHP(root, []byte(line)); (len(files) == 0) != tc.dropped {
			t.Errorf("%s (%s bytes) %q: listed %v, want dropped %v", tc.file, tc.size, tc.content, files, tc.dropped)
		}
	}
}

func TestParseDebugLog(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	long := strings.Repeat("x", 400)
	out := "4012345\n" +
		"[20-Oct-2026 08:00:00 UTC] PHP Fatal error:  Old one in /a.php:1\n" +
		"[30-Oct-2026 09:15:00 UTC] PHP Parse error:  syntax error, unexpected '}' in /b.php on line 3\n" +
		"PHP Fatal error:  without a date\n" +
		"[31-Oct-2026 02:14:07 UTC] PHP Fatal error:  Uncaught Error: Call to undefined function wc_get_order() " + long + "\n"
	got, err := remote.ParseDebugLog([]byte(out), now)
	if err != nil {
		t.Fatal(err)
	}
	want := remote.DebugLogRead{Exists: true, Size: 4012345, Recent: 2,
		Last:    time.Date(2026, 10, 31, 2, 14, 7, 0, time.UTC),
		Message: ("PHP Fatal error:  Uncaught Error: Call to undefined function wc_get_order() " + long)[:300]}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseDebugLog() = %+v\nwant %+v", got, want)
	}

	if got, err := remote.ParseDebugLog(nil, now); err != nil || got.Exists {
		t.Errorf("ParseDebugLog() of no file = %+v, %v", got, err)
	}
	if got, err := remote.ParseDebugLog([]byte("  812\n"), now); err != nil || got != (remote.DebugLogRead{Exists: true, Size: 812}) {
		t.Errorf("ParseDebugLog() without errors = %+v, %v", got, err)
	}
	if _, err := remote.ParseDebugLog([]byte("lots\n"), now); err == nil {
		t.Error("ParseDebugLog() of a size that is not a number: no error")
	}
}

func TestParseErrorLines(t *testing.T) {
	fatal := "[31-Oct-2026 02:14:07 UTC] PHP Fatal error:  Uncaught Error: x() in /a/b.php:3"
	thrown := "  thrown in /a/b.php on line 3"
	got, err := remote.ParseErrorLines([]byte("4012345\n" + fatal + "\r\n" + thrown + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := remote.ErrorLinesRead{Exists: true, Size: 4012345, Lines: []string{fatal, thrown}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseErrorLines() = %+v\nwant %+v", got, want)
	}
	if got.Whole() {
		t.Error("a file of 4 MB read whole")
	}

	// No file: nothing answered. An empty one: its size only.
	if got, err := remote.ParseErrorLines(nil); err != nil || got.Exists {
		t.Errorf("ParseErrorLines() of no file = %+v, %v", got, err)
	}
	if got, err := remote.ParseErrorLines([]byte("0\n")); err != nil || !got.Exists || got.Size != 0 || len(got.Lines) != 0 || !got.Whole() {
		t.Errorf("ParseErrorLines() of an empty file = %+v, %v", got, err)
	}
	if _, err := remote.ParseErrorLines([]byte("lots\n")); err == nil {
		t.Error("ParseErrorLines() of a size that is not a number: no error")
	}

	// One line more than kept: the oldest is left out, and the answer says older ones were.
	lines := make([]string, remote.ErrorLinesKept+1)
	for i := range lines {
		lines[i] = fmt.Sprintf("[31-Oct-2026 02:14:07 UTC] PHP Warning:  w%d in /a.php on line %d", i, i)
	}
	got, err = remote.ParseErrorLines([]byte("812\n" + strings.Join(lines, "\n") + "\n"))
	if err != nil || !got.More || len(got.Lines) != remote.ErrorLinesKept || got.Lines[0] != lines[1] {
		t.Errorf("ParseErrorLines() of too many lines: more %v, %d lines, first %q, %v", got.More, len(got.Lines), got.Lines[0], err)
	}
}

// ErrorLines's answer on a log of 1.1 MB written by PHP 8.4 under Apache (the bench's image), the
// same from GNU and BusyBox: the notices before it and the line cut by the tail left out, the "thrown
// in" lines kept, bytes that are not UTF-8 kept.
func TestParseRecordedErrorLines(t *testing.T) {
	out, err := os.ReadFile("testdata/error-lines.out")
	if err != nil {
		t.Fatal(err)
	}
	got, err := remote.ParseErrorLines(out)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Exists || got.Size != 1110504 || got.Whole() || got.More || len(got.Lines) != 16 {
		t.Fatalf("ParseErrorLines() = exists %v, size %d, whole %v, more %v, %d lines", got.Exists, got.Size, got.Whole(), got.More, len(got.Lines))
	}
	s := errorlog.Read(got.Lines, "/var/www/html")
	if s.Lines != 13 || len(s.Groups) != 13 {
		t.Errorf("%d lines, %d groups, want 13 and 13", s.Lines, len(s.Groups))
	}
	for _, g := range s.Groups {
		if strings.HasPrefix(g.Message, "Uncaught RuntimeException: Mailbox full") && (g.File != "gen.php" || g.Line != 13) {
			t.Errorf("the exception's place from \"thrown in\": %s:%d", g.File, g.Line)
		}
		if strings.Contains(g.Message, "Notice") || strings.Contains(g.Message, "marie@") {
			t.Errorf("crossed: %q", g.Message)
		}
	}
}
