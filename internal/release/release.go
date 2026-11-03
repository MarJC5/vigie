// Package release signs a release of Vigie (ADR-0038): the key pair made once, each artifact signed
// over its SHA-256, the SHA256SUMS list and update.json, the manifest Wails' endpoint provider reads.
// It runs on the owner's Mac, never in CI: the private key never leaves it.
package release

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Repository is where releases are published; an artifact's address is under its tag.
const Repository = "https://github.com/MarJC5/vigie"

// NewKeys makes the key pair, each as one line of base64: the public key (32 bytes) for the app,
// the private key (64 bytes) for the owner's Mac.
func NewKeys(random io.Reader) (public, private string, err error) {
	pub, priv, err := ed25519.GenerateKey(random)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(pub), base64.StdEncoding.EncodeToString(priv), nil
}

// ParsePublic reads a public key written by NewKeys.
func ParsePublic(text string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(text))
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, errors.New("not an Ed25519 public key")
	}
	return ed25519.PublicKey(raw), nil
}

// ParsePrivate reads a private key written by NewKeys.
func ParsePrivate(text string) (ed25519.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(text))
	if err != nil || len(raw) != ed25519.PrivateKeySize {
		return nil, errors.New("not an Ed25519 private key")
	}
	return ed25519.PrivateKey(raw), nil
}

// Signed is an artifact of a release: its file, where it is published, for which system (none for
// the files the updater does not install), its size, SHA-256 and signature.
type Signed struct {
	Filename  string
	URL       string
	Platform  string
	Arch      string
	Filetype  string
	Size      int64
	Digest    []byte
	Signature []byte
}

// Sign reads an artifact, and signs its SHA-256 with the private key (Wails' "ed25519" verifier
// checks the signature over the digest).
func Sign(key ed25519.PrivateKey, version, path string) (Signed, error) {
	f, err := os.Open(path)
	if err != nil {
		return Signed{}, err
	}
	defer f.Close()
	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return Signed{}, err
	}
	name := filepath.Base(path)
	s := Signed{Filename: name, URL: fmt.Sprintf("%s/releases/download/v%s/%s", Repository, version, name), Size: size, Digest: h.Sum(nil)}
	s.Signature = ed25519.Sign(key, s.Digest)
	s.Platform, s.Arch, s.Filetype = target(name)
	return s, nil
}

// target tells what the updater installs from a file: the zipped app on macOS (universal: any
// arch), the AppImage on Linux; nothing for the DMG and the packages.
func target(name string) (platform, arch, filetype string) {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, "-macos.zip"):
		return "darwin", "", "zip"
	case strings.HasSuffix(lower, ".appimage") && strings.Contains(lower, "x86_64"):
		return "linux", "amd64", "appimage"
	case strings.HasSuffix(lower, ".appimage") && strings.Contains(lower, "aarch64"):
		return "linux", "arm64", "appimage"
	}
	return "", "", ""
}

// Checksums is the SHA256SUMS list, as shasum -a 256 writes it.
func Checksums(signed []Signed) []byte {
	var b strings.Builder
	for _, s := range signed {
		fmt.Fprintf(&b, "%s  %s\n", hex.EncodeToString(s.Digest), s.Filename)
	}
	return []byte(b.String())
}

// manifest is Wails' Update Manifest, schema 1 (pkg/updater/providers/endpoint).
type manifest struct {
	SchemaVersion int        `json:"schemaVersion"`
	Version       string     `json:"version"`
	Name          string     `json:"name"`
	Notes         string     `json:"notes"`
	PublishedAt   string     `json:"publishedAt"`
	Artifacts     []artifact `json:"artifacts"`
}

type artifact struct {
	URL           string `json:"url"`
	Filename      string `json:"filename"`
	Filetype      string `json:"filetype"`
	Size          int64  `json:"size"`
	Platform      string `json:"platform"`
	Arch          string `json:"arch,omitempty"`
	DigestAlgo    string `json:"digestAlgo"`
	Digest        string `json:"digest"`
	SignatureAlgo string `json:"signatureAlgo"`
	Signature     string `json:"signature"`
}

// Manifest writes update.json: the version, its notes, and the artifacts the updater installs,
// each with its digest and signature in base64.
func Manifest(version, notes string, published time.Time, signed []Signed) ([]byte, error) {
	m := manifest{SchemaVersion: 1, Version: version, Name: "Vigie " + version, Notes: notes,
		PublishedAt: published.UTC().Format(time.RFC3339), Artifacts: []artifact{}}
	for _, s := range signed {
		if s.Platform == "" {
			continue
		}
		m.Artifacts = append(m.Artifacts, artifact{
			URL: s.URL, Filename: s.Filename, Filetype: s.Filetype, Size: s.Size, Platform: s.Platform, Arch: s.Arch,
			DigestAlgo: "sha256", Digest: base64.StdEncoding.EncodeToString(s.Digest),
			SignatureAlgo: "ed25519", Signature: base64.StdEncoding.EncodeToString(s.Signature),
		})
	}
	if len(m.Artifacts) == 0 {
		return nil, errors.New("no artifact the updater installs: a -macos.zip or an AppImage")
	}
	return json.MarshalIndent(m, "", "  ")
}

// Notes is the section of a version in a changelog kept as Keep a Changelog writes it: what follows
// its heading ("## [1.2.0] - 2026-11-14"), up to the next version or the links at the end.
func Notes(changelog, version string) (string, error) {
	var lines []string
	in := false
	for line := range strings.SplitSeq(changelog, "\n") {
		if heading, ok := strings.CutPrefix(line, "## "); ok {
			if in {
				break
			}
			in = strings.HasPrefix(heading, "["+version+"]")
			continue
		}
		if in && strings.HasPrefix(line, "[") && strings.Contains(line, "]: ") {
			break
		}
		if in {
			lines = append(lines, line)
		}
	}
	notes := strings.TrimSpace(strings.Join(lines, "\n"))
	if notes == "" {
		return "", fmt.Errorf("the changelog has no notes under ## [%s]", version)
	}
	return absoluteLinks(notes), nil
}

// markdownLink is a Markdown link, its target in the group.
var markdownLink = regexp.MustCompile(`\]\(([^)\s]+)\)`)

// absoluteLinks points the links of the changelog relative to the repository (docs/updates.md) at
// its files on main: the release page and the window resolve them from elsewhere. Addresses and
// anchors stay as they are.
func absoluteLinks(notes string) string {
	return markdownLink.ReplaceAllStringFunc(notes, func(link string) string {
		target := markdownLink.FindStringSubmatch(link)[1]
		if strings.Contains(target, ":") || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "/") {
			return link
		}
		return "](" + Repository + "/blob/main/" + strings.TrimPrefix(target, "./") + ")"
	})
}
