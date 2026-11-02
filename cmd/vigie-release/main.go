// Command vigie-release signs a release of Vigie on the owner's Mac (ADR-0038): it makes the key
// pair once, then signs the artifacts of each version and writes SHA256SUMS and update.json; make
// release also takes a version's notes from the changelog with it. It is not part of the app, and
// never runs in CI.
//
//	vigie-release keygen <folder>
//	vigie-release notes -version <x.y.z> <changelog>
//	vigie-release sign -key <file> -version <x.y.z> [-notes <file>] [-base <url>] <artifact>…
package main

import (
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MarJC5/vigie/internal/release"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, time.Now))
}

func run(args []string, out, errOut io.Writer, now func() time.Time) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "usage: vigie-release keygen <folder> | notes -version <x.y.z> <changelog> | sign -key <file> -version <x.y.z> [-notes <file>] [-base <url>] <artifact>…")
		return 2
	}
	var err error
	switch args[0] {
	case "keygen":
		err = keygen(args[1:], out)
	case "notes":
		err = notes(args[1:], out)
	case "sign":
		err = sign(args[1:], out, now)
	default:
		err = fmt.Errorf("unknown command %q", args[0])
	}
	if err != nil {
		fmt.Fprintln(errOut, "vigie-release:", err)
		return 1
	}
	return 0
}

// keygen writes the private key, readable by the owner only, and the public key, which it also
// prints for the app; it never overwrites a key.
func keygen(args []string, out io.Writer) error {
	if len(args) != 1 {
		return errors.New("keygen takes the folder to write the keys in, outside the repository")
	}
	dir := args[0]
	private, public := filepath.Join(dir, "vigie-release.key"), filepath.Join(dir, "vigie-release.pub")
	for _, p := range []string{private, public} {
		if _, err := os.Stat(p); err == nil {
			return fmt.Errorf("%s exists: a key is never overwritten", p)
		}
	}
	pub, priv, err := release.NewKeys(rand.Reader)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(private, []byte(priv+"\n"), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(public, []byte(pub+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "private key: %s (keep it, never in the repository)\npublic key, for the app:\n%s\n", private, pub)
	return nil
}

// notes prints a version's section of the changelog: the release's notes.
func notes(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("notes", flag.ContinueOnError)
	version := fs.String("version", "", "the version, without v")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *version == "" || fs.NArg() != 1 {
		return errors.New("notes needs -version and the changelog")
	}
	text, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		return err
	}
	section, err := release.Notes(string(text), *version)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, section)
	return nil
}

// sign signs each artifact and writes SHA256SUMS and update.json next to the first one.
func sign(args []string, out io.Writer, now func() time.Time) error {
	fs := flag.NewFlagSet("sign", flag.ContinueOnError)
	keyFile := fs.String("key", "", "the private key written by keygen")
	version := fs.String("version", "", "the version, without v")
	notesFile := fs.String("notes", "", "the release notes, Markdown")
	base := fs.String("base", "", "where the artifacts are served, to try an update from a local server; the release's tag on GitHub otherwise")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *keyFile == "" || *version == "" || fs.NArg() == 0 {
		return errors.New("sign needs -key, -version and the artifacts")
	}
	text, err := os.ReadFile(*keyFile)
	if err != nil {
		return err
	}
	key, err := release.ParsePrivate(string(text))
	if err != nil {
		return err
	}
	notes := ""
	if *notesFile != "" {
		b, err := os.ReadFile(*notesFile)
		if err != nil {
			return err
		}
		notes = string(b)
	}
	var signed []release.Signed
	for _, path := range fs.Args() {
		s, err := release.Sign(key, *version, path)
		if err != nil {
			return err
		}
		if *base != "" {
			s.URL = strings.TrimSuffix(*base, "/") + "/" + s.Filename
		}
		signed = append(signed, s)
	}
	manifest, err := release.Manifest(*version, notes, now(), signed)
	if err != nil {
		return err
	}
	dir := filepath.Dir(fs.Arg(0))
	if err := os.WriteFile(filepath.Join(dir, "SHA256SUMS"), release.Checksums(signed), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "update.json"), append(manifest, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "signed %d artifacts; wrote %s and %s\n", len(signed), filepath.Join(dir, "SHA256SUMS"), filepath.Join(dir, "update.json"))
	return nil
}
