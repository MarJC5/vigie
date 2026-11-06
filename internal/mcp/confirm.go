package mcp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

// confirmWindow is how long a preview's confirmation holds (ADR-0043, point 5).
const confirmWindow = 5 * time.Minute

// pending is a preview waiting for the user's yes: the call it was made for, and what it showed.
type pending struct {
	tool, args string
	digest     string // of what the preview showed, so that only what the user saw runs
	expires    time.Time
}

// confirmations hold the previews waiting for a yes, in this process only: a client that starts
// vigie mcp again asks for a new preview. Each confirmation holds once.
type confirmations struct {
	mu   sync.Mutex
	key  []byte // signs the state a client sends back with the user's answer (MRTR)
	open map[string]pending
}

func newConfirmations() *confirmations {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err) // the system's random source never fails on the systems Vigie runs on
	}
	return &confirmations{key: key, open: map[string]pending{}}
}

// issue opens a confirmation for a call and what its preview showed.
func (c *confirmations) issue(now time.Time, tool, args, digest string) (string, time.Time) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	token := hex.EncodeToString(b)
	c.mu.Lock()
	defer c.mu.Unlock()
	for t, p := range c.open {
		if now.After(p.expires) {
			delete(c.open, t)
		}
	}
	expires := now.Add(confirmWindow)
	c.open[token] = pending{tool: tool, args: args, digest: digest, expires: expires}
	return token, expires
}

// take closes a confirmation and returns what its preview showed; ok is false when it does not
// hold: unknown, used, older than five minutes, or for another tool or other arguments.
func (c *confirmations) take(now time.Time, token, tool, args string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, found := c.open[token]
	delete(c.open, token)
	if !found || p.tool != tool || p.args != args || now.After(p.expires) {
		return "", false
	}
	return p.digest, true
}

// sign is the state sent with Vigie's question: the confirmation, signed, since a client sends it
// back and must not be able to forge it.
func (c *confirmations) sign(token string) string {
	return token + "." + c.mac(token)
}

// verify returns the confirmation of a state Vigie signed.
func (c *confirmations) verify(state string) (string, bool) {
	token, mac, found := strings.Cut(state, ".")
	if !found || !hmac.Equal([]byte(mac), []byte(c.mac(token))) {
		return "", false
	}
	return token, true
}

func (c *confirmations) mac(token string) string {
	h := hmac.New(sha256.New, c.key)
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

// digestOf names what a preview showed.
func digestOf(lines []string) string {
	h := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(h[:])
}
