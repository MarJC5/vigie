package sshx

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrBadHostKey is returned for a key line that is not a supported, well-formed public key.
	ErrBadHostKey = errors.New("invalid host key")
	// ErrNoHostKey is returned when the server presented no key.
	ErrNoHostKey = errors.New("no host key received")
	// ErrBadHostName is returned for a name that could be read as an option or would change the
	// meaning of a known_hosts line.
	ErrBadHostName = errors.New("invalid host name")
	// ErrBadPath is returned for a data directory that cannot be written in an ssh option.
	ErrBadPath = errors.New("invalid path")
)

// HostKey is one public key of an SSH server, as ssh-keyscan prints it and known_hosts stores it.
type HostKey struct {
	Type string // for example "ssh-ed25519"
	Key  string // base64 of the public key
}

// The key types Vigie asks for (ed25519, ecdsa, rsa).
var hostKeyTypes = map[string]bool{
	"ssh-ed25519":         true,
	"ecdsa-sha2-nistp256": true,
	"ecdsa-sha2-nistp384": true,
	"ecdsa-sha2-nistp521": true,
	"ssh-rsa":             true,
}

// Fingerprint returns the fingerprint OpenSSH shows: "SHA256:" and the unpadded base64 of the
// SHA-256 of the key.
func (k HostKey) Fingerprint() (string, error) {
	blob, err := k.blob()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(blob)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:]), nil
}

// blob decodes the key and checks that it starts with its own type name, as every SSH public
// key does.
func (k HostKey) blob() ([]byte, error) {
	if !hostKeyTypes[k.Type] {
		return nil, fmt.Errorf("type %q: %w", k.Type, ErrBadHostKey)
	}
	blob, err := base64.StdEncoding.DecodeString(k.Key)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", k.Type, ErrBadHostKey)
	}
	if len(blob) < 4 {
		return nil, fmt.Errorf("%s: %w", k.Type, ErrBadHostKey)
	}
	n := binary.BigEndian.Uint32(blob)
	if uint64(n) > uint64(len(blob)-4) || string(blob[4:4+n]) != k.Type {
		return nil, fmt.Errorf("%s: key of another type: %w", k.Type, ErrBadHostKey)
	}
	return blob, nil
}

// ParseKeyScan reads the output of ssh-keyscan: "name type key" lines, "#" comments skipped.
func ParseKeyScan(out []byte) ([]HostKey, error) {
	var keys []HostKey
	for line := range strings.Lines(string(out)) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, fmt.Errorf("%q: %w", line, ErrBadHostKey)
		}
		key := HostKey{Type: fields[1], Key: fields[2]}
		if _, err := key.blob(); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, nil
}

// KnownHostsName is the name ssh looks a host up by: the host name, in brackets with the port
// when the port is not 22.
func KnownHostsName(host string, port int) string {
	if port == 0 || port == 22 {
		return host
	}
	return "[" + host + "]:" + strconv.Itoa(port)
}

// KnownHostsFile returns Vigie's own known_hosts in its data directory. The path goes into an ssh
// option between double quotes, so it cannot contain one.
func KnownHostsFile(dataDir string) (string, error) {
	if strings.ContainsAny(dataDir, "\"\n") {
		return "", fmt.Errorf("%q: %w", dataDir, ErrBadPath)
	}
	return filepath.Join(dataDir, "known_hosts"), nil
}

// checkHostName refuses what OpenSSH itself never records (sshconnect.c), what would split the
// line, and what ssh tools would read as an option.
func checkHostName(name string) error {
	if name == "" || strings.HasPrefix(name, "-") || strings.HasPrefix(name, "!") ||
		strings.ContainsAny(name, "@?*#|'\"\\, \t\r\n") {
		return fmt.Errorf("%q: %w", name, ErrBadHostName)
	}
	return nil
}

// checkKnownHostsName accepts a host name, or "[host]:port".
func checkKnownHostsName(name string) error {
	if host, rest, ok := strings.Cut(strings.TrimPrefix(name, "["), "]:"); ok && strings.HasPrefix(name, "[") {
		if _, err := strconv.Atoi(rest); err != nil {
			return fmt.Errorf("%q: %w", name, ErrBadHostName)
		}
		name = host
	}
	if strings.ContainsAny(name, "[]") {
		return fmt.Errorf("%q: %w", name, ErrBadHostName)
	}
	return checkHostName(name)
}

// Trust records key for name in Vigie's known_hosts file (created 0600), replacing the keys that
// name had: the user's own SSH files are never written.
func Trust(file, name string, key HostKey) error {
	if err := checkKnownHostsName(name); err != nil {
		return err
	}
	if _, err := key.blob(); err != nil {
		return err
	}
	old, err := os.ReadFile(file)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var buf bytes.Buffer
	for line := range strings.Lines(string(old)) {
		if first, _, _ := strings.Cut(line, " "); first != name {
			buf.WriteString(line)
		}
	}
	fmt.Fprintf(&buf, "%s %s %s\n", name, key.Type, key.Key)

	tmp, err := os.CreateTemp(filepath.Dir(file), ".known_hosts-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), file)
}

// TrustedKeys returns the keys Vigie's known_hosts file trusts for name; none when the file or
// the name is missing. Lines it would not write itself are skipped.
func TrustedKeys(file, name string) ([]HostKey, error) {
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var keys []HostKey
	for line := range strings.Lines(string(data)) {
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[0] != name {
			continue
		}
		key := HostKey{Type: fields[1], Key: fields[2]}
		if _, err := key.blob(); err == nil {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

// KeyScanner asks a server for its host keys with the system ssh-keyscan. Nothing is trusted
// until the user accepts a fingerprint.
type KeyScanner struct {
	Binary  string        // path of ssh-keyscan; "ssh-keyscan" when empty
	Timeout time.Duration // connection timeout; 5 seconds when zero
}

// Scan returns the ed25519, ecdsa and rsa keys of the server at host and port.
func (s KeyScanner) Scan(ctx context.Context, host string, port int) ([]HostKey, error) {
	if err := checkHostName(host); err != nil {
		return nil, err
	}
	binary := s.Binary
	if binary == "" {
		binary = "ssh-keyscan"
	}
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if port == 0 {
		port = 22
	}
	ctx, cancel := context.WithTimeout(ctx, timeout+5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary,
		"-T", seconds(timeout), "-p", strconv.Itoa(port), "-t", "ed25519,ecdsa,rsa", "--", host)
	cmd.WaitDelay = time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	keys, err := ParseKeyScan(stdout.Bytes())
	if err != nil {
		return nil, err
	}
	if len(keys) > 0 {
		return keys, nil
	}
	// No key: why, read from ssh-keyscan's messages like ssh's; silence is ErrNoHostKey.
	if msg := strings.TrimSpace(stderr.String()); msg != "" {
		return nil, &ConnectionError{Failure: Classify(stderr.Bytes()), Message: msg}
	}
	if runErr != nil {
		return nil, fmt.Errorf("ssh-keyscan: %w", runErr)
	}
	return nil, fmt.Errorf("%w from %s", ErrNoHostKey, KnownHostsName(host, port))
}
