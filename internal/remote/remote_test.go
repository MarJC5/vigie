package remote_test

import (
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

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
			`tail -c +1201 '/home/u/sites/a/wp-content/debug.log' | grep -m 3 'PHP Fatal error'`},
		{"backups", func() (string, error) { return remote.ListBackups(site) }, `ls -1 '.vigie/backups/01a18704-356b-77be-ab41-b92a0d67a6bc'`},
		{"remove a backup", func() (string, error) { return remote.RemoveBackup(dir) }, `rm -rf '` + dir + `'`},
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
		"debug log not absolute": func() (string, error) { return remote.FileSize("wp-content/debug.log") },
		"a negative offset":      func() (string, error) { return remote.FatalsSince("/a/debug.log", -1) },
		"a dump out of a backup": func() (string, error) { return remote.Uncompress("/tmp/x.sql.gz") },
		"home as a root":         func() (string, error) { return remote.RealRoot("~") },
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
	fatal := "[29-Oct-2026 14:12:18 UTC] PHP Fatal error:  Uncaught Error: Call to undefined function x() in /a/b.php:6"
	if got := remote.ParseFatals(sshx.Result{Stdout: []byte(fatal + "\n")}); !slices.Equal(got, []string{fatal}) {
		t.Errorf("ParseFatals = %q", got)
	}
	// grep finds nothing: exit 1, no line.
	if got := remote.ParseFatals(sshx.Result{ExitCode: 1}); len(got) != 0 {
		t.Errorf("ParseFatals(none) = %q", got)
	}
}
