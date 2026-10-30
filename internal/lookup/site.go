package lookup

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Why a certificate is not valid; empty when it is.
const (
	Expired          = "expired"
	WrongName        = "wrong-name"        // issued for another name
	UnknownAuthority = "unknown-authority" // self-signed, or from an authority the system does not trust
	Invalid          = "invalid"           // any other reason the system refuses it
)

// siteTimeout bounds a connection to a site and its TLS handshake, as one.
const siteTimeout = 15 * time.Second

// Certificate is a site's TLS certificate as a visitor's browser gets it.
type Certificate struct {
	Host     string
	NotAfter time.Time
	Issuer   string
	Problem  string // empty when valid, else Expired, WrongName, UnknownAuthority or Invalid
}

// Certificate reads the certificate a site serves on port 443, with the host name it is asked
// for; a certificate the system refuses is still read, and Problem says why.
func (c Client) Certificate(ctx context.Context, host string) (Certificate, error) {
	if host == "" || strings.ContainsAny(host, ":/ ") {
		return Certificate{}, fmt.Errorf("host %q: %w", host, ErrBadInput)
	}
	if c.Allowed == nil || !c.Allowed(Site) {
		return Certificate{}, fmt.Errorf("%s: %w", Site, ErrOff)
	}
	ctx, cancel := context.WithTimeout(ctx, siteTimeout)
	defer cancel()
	dial := c.Dial
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}
	conn, err := dial(ctx, "tcp", net.JoinHostPort(host, "443"))
	if err != nil {
		return Certificate{}, err
	}
	defer conn.Close()
	tc := tls.Client(conn, &tls.Config{ServerName: host, RootCAs: c.Roots, Time: c.Now, MinVersion: tls.VersionTLS12})
	result := Certificate{Host: host}
	err = tc.HandshakeContext(ctx)
	var invalid *tls.CertificateVerificationError
	switch {
	case err == nil:
		leaf := tc.ConnectionState().PeerCertificates[0]
		result.NotAfter, result.Issuer = leaf.NotAfter, issuer(leaf)
		return result, nil
	case errors.As(err, &invalid) && len(invalid.UnverifiedCertificates) > 0:
		leaf := invalid.UnverifiedCertificates[0]
		result.NotAfter, result.Issuer, result.Problem = leaf.NotAfter, issuer(leaf), problemOf(invalid.Err)
		return result, nil
	}
	return Certificate{}, err
}

func problemOf(err error) string {
	var invalid x509.CertificateInvalidError
	var name x509.HostnameError
	var authority x509.UnknownAuthorityError
	switch {
	case errors.As(err, &invalid) && invalid.Reason == x509.Expired:
		return Expired
	case errors.As(err, &name):
		return WrongName
	case errors.As(err, &authority):
		return UnknownAuthority
	}
	return Invalid
}

func issuer(cert *x509.Certificate) string {
	if len(cert.Issuer.Organization) > 0 {
		return cert.Issuer.Organization[0]
	}
	return cert.Issuer.CommonName
}

// DebugLogCheck is whether a site's debug.log can be downloaded, as an attacker would try.
type DebugLogCheck struct {
	URL    string
	Status int
	Public bool
}

// DebugLog asks a site for the first byte of its debug.log, without following redirects; served
// (200 or 206) and not an HTML page means anyone can read it. Nothing of the log is kept.
func (c Client) DebugLog(ctx context.Context, home string) (DebugLogCheck, error) {
	base, err := url.Parse(home)
	if err != nil || (base.Scheme != "https" && base.Scheme != "http") || base.Host == "" {
		return DebugLogCheck{}, fmt.Errorf("site address %q: %w", home, ErrBadInput)
	}
	if c.Allowed == nil || !c.Allowed(Site) {
		return DebugLogCheck{}, fmt.Errorf("%s: %w", Site, ErrOff)
	}
	check := DebugLogCheck{URL: strings.TrimSuffix(base.String(), "/") + "/wp-content/debug.log"}
	ctx, cancel := context.WithTimeout(ctx, siteTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, check.URL, nil)
	if err != nil {
		return DebugLogCheck{}, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Range", "bytes=0-0")
	noRedirect := *c.HTTP
	noRedirect.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := noRedirect.Do(req)
	if err != nil {
		return DebugLogCheck{}, err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1))
	check.Status = res.StatusCode
	served := res.StatusCode == http.StatusOK || res.StatusCode == http.StatusPartialContent
	// Some hosts answer a missing file with their own page and a 200.
	page := strings.HasPrefix(strings.ToLower(res.Header.Get("Content-Type")), "text/html")
	check.Public = served && !page
	return check, nil
}
