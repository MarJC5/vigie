package release_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/endpoint"

	"github.com/MarJC5/vigie/internal/release"
)

// The key pair: the private key written for the owner's Mac, the public one for the app, both read
// back as they were (ADR-0038).
func TestKeysRoundTrip(t *testing.T) {
	pub, priv, err := release.NewKeys(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	p, err := release.ParsePublic(pub)
	if err != nil {
		t.Fatal(err)
	}
	k, err := release.ParsePrivate(priv)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(k.Public().(ed25519.PublicKey), p) {
		t.Fatal("the private key does not match the public one")
	}
	if _, err := release.ParsePrivate(pub); err == nil {
		t.Fatal("a public key read as a private one")
	}
}

// Each artifact signed over its SHA-256, listed in SHA256SUMS; the updater's artifacts in
// update.json, which Wails' endpoint provider reads with the digest and the signature.
func TestTheManifestWailsReads(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	app := write("Vigie-0.6.0-macos.zip", "the app")
	image := write("Vigie-0.6.0-x86_64.AppImage", "the image")
	dmg := write("Vigie-0.6.0.dmg", "the dmg")
	pubText, privText, _ := release.NewKeys(rand.Reader)
	pub, _ := release.ParsePublic(pubText)
	priv, _ := release.ParsePrivate(privText)

	var signed []release.Signed
	for _, path := range []string{app, image, dmg} {
		s, err := release.Sign(priv, "0.6.0", path)
		if err != nil {
			t.Fatal(err)
		}
		signed = append(signed, s)
	}
	sum := sha256.Sum256([]byte("the app"))
	if !bytes.Equal(signed[0].Digest, sum[:]) || !ed25519.Verify(pub, sum[:], signed[0].Signature) {
		t.Fatal("the app's digest or signature is wrong")
	}
	if signed[0].Platform != "darwin" || signed[0].Arch != "" || signed[1].Platform != "linux" || signed[1].Arch != "amd64" || signed[2].Platform != "" {
		t.Fatalf("platforms = %+v", signed)
	}
	if !strings.HasSuffix(signed[0].URL, "/releases/download/v0.6.0/Vigie-0.6.0-macos.zip") {
		t.Fatalf("URL = %s", signed[0].URL)
	}

	sums := string(release.Checksums(signed))
	if want := hex.EncodeToString(sum[:]) + "  Vigie-0.6.0-macos.zip\n"; !strings.HasPrefix(sums, want) || strings.Count(sums, "\n") != 3 {
		t.Fatalf("SHA256SUMS = %q", sums)
	}

	manifest, err := release.Manifest("0.6.0", "- Several items selected", time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC), signed)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(manifest) }))
	defer server.Close()
	provider, err := endpoint.New(endpoint.Config{URL: server.URL + "/update.json"})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ platform, arch, file string }{
		{"darwin", "arm64", "Vigie-0.6.0-macos.zip"},
		{"darwin", "amd64", "Vigie-0.6.0-macos.zip"},
		{"linux", "amd64", "Vigie-0.6.0-x86_64.AppImage"},
	} {
		rel, err := provider.Check(context.Background(), updater.CheckRequest{CurrentVersion: "0.5.0", Platform: c.platform, Arch: c.arch})
		if err != nil || rel == nil {
			t.Fatalf("%s/%s: Check() = %v, %v", c.platform, c.arch, rel, err)
		}
		if rel.Version != "0.6.0" || rel.Artifact.Filename != c.file || rel.Verification == nil ||
			rel.Verification.SignatureAlgo != "ed25519" || !ed25519.Verify(pub, rel.Verification.Digest, rel.Verification.Signature) {
			t.Fatalf("%s/%s: release = %+v", c.platform, c.arch, rel)
		}
	}
}

// The notes of a version are its section of the changelog, without the next one nor the links.
func TestNotesFromTheChangelog(t *testing.T) {
	changelog := "# Changelog\n\n## [Unreleased]\n\n### Added\n\n- Next.\n\n" +
		"## [1.1.0] - 2026-11-20\n\n### Fixed\n\n- A fix.\n\n" +
		"## [1.0.0] - 2026-11-14\n\n### Added\n\n- The first release.\n\n" +
		"[1.1.0]: https://github.com/MarJC5/vigie/compare/v1.0.0...v1.1.0\n[1.0.0]: https://github.com/MarJC5/vigie/releases/tag/v1.0.0\n"
	cases := map[string]string{
		"1.1.0": "### Fixed\n\n- A fix.",
		"1.0.0": "### Added\n\n- The first release.",
	}
	for version, want := range cases {
		got, err := release.Notes(changelog, version)
		if err != nil || got != want {
			t.Errorf("Notes(%s) = %q, %v; want %q", version, got, err, want)
		}
	}
	for _, version := range []string{"1.2.0", "1.0"} {
		if _, err := release.Notes(changelog, version); err == nil {
			t.Errorf("Notes(%s): no error for a version the changelog does not have", version)
		}
	}
}
