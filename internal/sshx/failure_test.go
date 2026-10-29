package sshx_test

import (
	"os"
	"testing"

	"github.com/MarJC5/vigie/internal/sshx"
)

// Messages captured from OpenSSH 10 (macOS and Debian), with fictional names and paths.
func TestClassifyOpenSSHMessages(t *testing.T) {
	for file, want := range map[string]sshx.Failure{
		"refused":          sshx.FailureRefused,
		"unresolved":       sshx.FailureUnresolved,
		"timed-out-macos":  sshx.FailureTimedOut,
		"timed-out-linux":  sshx.FailureTimedOut,
		"host-key-unknown": sshx.FailureHostKeyUnknown,
		"host-key-changed": sshx.FailureHostKeyChanged,
		"auth-refused":     sshx.FailureAuthRefused,
		"key-file-missing": sshx.FailureKeyFileMissing,
		"other":            sshx.FailureOther,
	} {
		stderr, err := os.ReadFile("testdata/stderr/" + file + ".txt")
		if err != nil {
			t.Fatal(err)
		}
		if got := sshx.Classify(stderr); got != want {
			t.Errorf("Classify(%s) = %q, want %q", file, got, want)
		}
	}
}
