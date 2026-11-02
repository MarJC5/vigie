package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// keygen writes the two keys once, the private one readable by the owner only; sign writes
// SHA256SUMS and update.json next to the artifacts.
func TestKeygenThenSign(t *testing.T) {
	keys := filepath.Join(t.TempDir(), "keys")
	var out, errOut bytes.Buffer
	if code := run([]string{"keygen", keys}, &out, &errOut, time.Now); code != 0 {
		t.Fatalf("keygen = %d: %s", code, errOut.String())
	}
	info, err := os.Stat(filepath.Join(keys, "vigie-release.key"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("private key: %v, %v", info, err)
	}
	if code := run([]string{"keygen", keys}, &out, &errOut, time.Now); code == 0 || !strings.Contains(errOut.String(), "never overwritten") {
		t.Fatalf("a second keygen = %d: %s", code, errOut.String())
	}

	dist := t.TempDir()
	for _, name := range []string{"Vigie-0.6.0-macos.zip", "Vigie-0.6.0.dmg"} {
		if err := os.WriteFile(filepath.Join(dist, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	errOut.Reset()
	args := []string{"sign", "-key", filepath.Join(keys, "vigie-release.key"), "-version", "0.6.0",
		filepath.Join(dist, "Vigie-0.6.0-macos.zip"), filepath.Join(dist, "Vigie-0.6.0.dmg")}
	if code := run(args, &out, &errOut, func() time.Time { return time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC) }); code != 0 {
		t.Fatalf("sign = %d: %s", code, errOut.String())
	}
	sums, _ := os.ReadFile(filepath.Join(dist, "SHA256SUMS"))
	if strings.Count(string(sums), "\n") != 2 {
		t.Fatalf("SHA256SUMS = %q", sums)
	}
	raw, _ := os.ReadFile(filepath.Join(dist, "update.json"))
	var m struct {
		Version   string `json:"version"`
		Artifacts []struct {
			Filename string `json:"filename"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(raw, &m); err != nil || m.Version != "0.6.0" || len(m.Artifacts) != 1 || m.Artifacts[0].Filename != "Vigie-0.6.0-macos.zip" {
		t.Fatalf("update.json = %s, %v", raw, err)
	}
}
