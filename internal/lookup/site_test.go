package lookup_test

import (
	"context"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/lookup"
)

// tlsSite serves a certificate for example.com (httptest's), reached whatever the host dialled.
func tlsSite(t *testing.T) (*httptest.Server, func(ctx context.Context, network, addr string) (net.Conn, error)) {
	t.Helper()
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	dial := func(ctx context.Context, network, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, srv.Listener.Addr().String())
	}
	return srv, dial
}

func TestCertificateOfASite(t *testing.T) {
	srv, dial := tlsSite(t)
	c, _, now := client(nil)
	c.Dial = dial
	c.Roots = x509.NewCertPool()
	c.Roots.AddCert(srv.Certificate())
	ctx := context.Background()

	cert, err := c.Certificate(ctx, "example.com")
	if err != nil || cert.Problem != "" || !cert.NotAfter.Equal(srv.Certificate().NotAfter) {
		t.Fatalf("Certificate(example.com) = %+v, %v; want valid until %s", cert, err, srv.Certificate().NotAfter)
	}
	// Another name, served the same certificate.
	if cert, err := c.Certificate(ctx, "boulangerie-exemple.ch"); err != nil || cert.Problem != lookup.WrongName {
		t.Errorf("Certificate(another name) = %+v, %v; want %q", cert, err, lookup.WrongName)
	}
	// Past its end: still read, reported expired.
	*now = srv.Certificate().NotAfter.Add(time.Hour)
	if cert, err := c.Certificate(ctx, "example.com"); err != nil || cert.Problem != lookup.Expired || cert.NotAfter.IsZero() {
		t.Errorf("Certificate() after its end = %+v, %v; want %q with its date", cert, err, lookup.Expired)
	}
	// From an authority the machine does not trust.
	*now = srv.Certificate().NotBefore.Add(time.Hour)
	c.Roots = x509.NewCertPool()
	if cert, err := c.Certificate(ctx, "example.com"); err != nil || cert.Problem != lookup.UnknownAuthority {
		t.Errorf("Certificate() from an unknown authority = %+v, %v; want %q", cert, err, lookup.UnknownAuthority)
	}
}

func TestCertificateRefusals(t *testing.T) {
	c, _, _ := client(nil, lookup.Site)
	if _, err := c.Certificate(context.Background(), "example.com"); !errors.Is(err, lookup.ErrOff) {
		t.Errorf("Certificate() turned off = %v, want ErrOff", err)
	}
	c, _, _ = client(nil)
	for _, host := range []string{"", "example.com:8443", "a b"} {
		if _, err := c.Certificate(context.Background(), host); !errors.Is(err, lookup.ErrBadInput) {
			t.Errorf("Certificate(%q) = %v, want ErrBadInput", host, err)
		}
	}
}

func TestDebugLog(t *testing.T) {
	const log = "https://boulangerie-exemple.ch/wp-content/debug.log"
	for _, tc := range []struct {
		name   string
		answer answer
		public bool
	}{
		{"served", answer{status: 206, body: "[", contentType: "text/plain"}, true},
		{"served whole", answer{status: 200, body: "[30-Oct-2026", contentType: "application/octet-stream"}, true},
		{"refused", answer{status: 403, contentType: "text/html"}, false},
		{"missing", answer{status: 404, contentType: "text/html"}, false},
		{"redirected", answer{status: 301, location: "https://boulangerie-exemple.ch/"}, false},
		{"a soft 404 page", answer{status: 200, body: "<!doctype html>", contentType: "text/html; charset=UTF-8"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, r, _ := client(map[string]answer{log: tc.answer, "https://boulangerie-exemple.ch/": {status: 200, contentType: "text/plain"}})
			got, err := c.DebugLog(context.Background(), "https://boulangerie-exemple.ch")
			if err != nil || got.Public != tc.public || got.Status != tc.answer.status {
				t.Fatalf("DebugLog() = %+v, %v; want public %v", got, err, tc.public)
			}
			// One request, for its first byte only, never following a redirect.
			sent(t, r, log)
			if rng := r.requests[0].Header.Get("Range"); rng != "bytes=0-0" {
				t.Errorf("Range = %q, want bytes=0-0", rng)
			}
		})
	}
}

func TestDebugLogRefusals(t *testing.T) {
	c, r, _ := client(nil, lookup.Site)
	if _, err := c.DebugLog(context.Background(), "https://example.com"); !errors.Is(err, lookup.ErrOff) {
		t.Errorf("DebugLog() turned off = %v, want ErrOff", err)
	}
	c, r, _ = client(nil)
	for _, home := range []string{"", "ftp://example.com", "https://", "javascript:alert(1)"} {
		if _, err := c.DebugLog(context.Background(), home); !errors.Is(err, lookup.ErrBadInput) {
			t.Errorf("DebugLog(%q) = %v, want ErrBadInput", home, err)
		}
	}
	sent(t, r)
}
