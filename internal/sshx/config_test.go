package sshx_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/MarJC5/vigie/internal/sshx"
)

// writeFiles writes files under a home made for the test.
func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	home := t.TempDir()
	for name, content := range files {
		path := filepath.Join(home, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func TestTheNamesOfTheHostLinesAndOfTheFilesIncluded(t *testing.T) {
	home := writeFiles(t, map[string]string{
		".ssh/config": `# Hosts of the test
Host shared shared-old  # two names
    HostName ssh.example.test
    Port 2222
host=staging
Host = "quoted name" production
Host * !jump *.example.test web?
    ForwardAgent no
Include config.d/*.conf ~/other/ssh.conf missing/*
Match host shared exec "true"
    User nobody
Host shared
`,
		".ssh/config.d/b.conf": "Host from-b\n",
		".ssh/config.d/a.conf": "Host from-a\nInclude nested.conf\n",
		".ssh/nested.conf":     "HOST nested\n",
		"other/ssh.conf":       "Host from-home\n",
	})
	got, err := sshx.ConfigNames(home)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"shared", "shared-old", "staging", "quoted name", "production", "from-a", "nested", "from-b", "from-home"}
	if !slices.Equal(got, want) {
		t.Errorf("ConfigNames() =\n%q\nwant\n%q", got, want)
	}
}

func TestNoConfigListsNothing(t *testing.T) {
	got, err := sshx.ConfigNames(t.TempDir())
	if err != nil || len(got) != 0 {
		t.Errorf("ConfigNames(no file) = %q, %v", got, err)
	}
}

// A file that includes itself stops where ssh stops, instead of looping.
func TestIncludesInALoopStop(t *testing.T) {
	home := writeFiles(t, map[string]string{".ssh/config": "Host loop\nInclude config\n"})
	if _, err := sshx.ConfigNames(home); !errors.Is(err, sshx.ErrConfigDepth) {
		t.Errorf("ConfigNames(a loop) = %v, want ErrConfigDepth", err)
	}
}
