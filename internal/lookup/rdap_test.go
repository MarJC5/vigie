package lookup_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/lookup"
)

const (
	bootstrap = "https://data.iana.org/rdap/dns.json"
	verisign  = "https://rdap.verisign.com/com/v1/domain/"
)

func TestRegisteredDomain(t *testing.T) {
	for _, tc := range []struct{ host, want string }{
		{"wordpress.com", "wordpress.com"},
		{"www.wordpress.com", "wordpress.com"},
		{"streamsafe.misits.ch", "misits.ch"},
		{"a.b.example.co.uk", "example.co.uk"},
		{"site.github.io", "github.io"}, // a private suffix: the registry knows github.io
		{"Boulangerie-Exemple.CH.", "boulangerie-exemple.ch"},
		{"bücher.example", "xn--bcher-kva.example"},
	} {
		if got, err := lookup.RegisteredDomain(tc.host); err != nil || got != tc.want {
			t.Errorf("RegisteredDomain(%q) = %q, %v; want %q", tc.host, got, err, tc.want)
		}
	}
	for _, host := range []string{"", "com", "localhost", "192.168.1.10", "a b.ch"} {
		if got, err := lookup.RegisteredDomain(host); !errors.Is(err, lookup.ErrBadInput) {
			t.Errorf("RegisteredDomain(%q) = %q, %v; want ErrBadInput", host, got, err)
		}
	}
}

func TestDomainExpiryFromTheRegistry(t *testing.T) {
	c, r, _ := client(map[string]answer{
		bootstrap:                  {200, "rdap/bootstrap.json", ""},
		verisign + "wordpress.com": {200, "rdap/com-wordpress.json", ""},
	})
	got, err := c.DomainExpiry(context.Background(), "www.wordpress.com")
	if err != nil {
		t.Fatal(err)
	}
	// Only the registered domain leaves the machine, to the registry of its TLD.
	sent(t, r, bootstrap, verisign+"wordpress.com")
	if got.Domain != "wordpress.com" || got.Reason != "" || !got.Expires.Equal(time.Date(2033, 3, 3, 12, 13, 23, 0, time.UTC)) {
		t.Fatalf("DomainExpiry() = %+v", got)
	}
}

// SWITCH (.ch, .li) and DENIC (.de) publish no expiry date: nothing is asked.
func TestNoRequestWhereTheRegistryPublishesNothing(t *testing.T) {
	c, r, _ := client(nil)
	for _, host := range []string{"streamsafe.misits.ch", "example.li", "denic.de"} {
		got, err := c.DomainExpiry(context.Background(), host)
		if err != nil || got.Reason != lookup.NotPublished || !got.Expires.IsZero() {
			t.Errorf("DomainExpiry(%q) = %+v, %v; want not published", host, got, err)
		}
	}
	sent(t, r)
}

func TestDomainUnknownOrWithoutRegistry(t *testing.T) {
	c, r, _ := client(map[string]answer{
		bootstrap: {200, "rdap/bootstrap.json", ""},
		verisign + "zzz-nonexistent-vigie-test-1234.com": {404, "rdap/com-unknown.json", ""},
	})
	ctx := context.Background()
	got, err := c.DomainExpiry(ctx, "zzz-nonexistent-vigie-test-1234.com")
	if err != nil || got.Reason != lookup.NotFound {
		t.Fatalf("an unregistered domain = %+v, %v; want not found", got, err)
	}
	// .zz has no RDAP server in the bootstrap: only the bootstrap was asked, and it is kept.
	got, err = c.DomainExpiry(ctx, "example.zz")
	if err != nil || got.Reason != lookup.NoService {
		t.Fatalf("a TLD without RDAP = %+v, %v; want no service", got, err)
	}
	sent(t, r, bootstrap, verisign+"zzz-nonexistent-vigie-test-1234.com")
}

func TestDomainExpiryIsKeptForAWeek(t *testing.T) {
	c, r, now := client(map[string]answer{
		bootstrap:                  {200, "rdap/bootstrap.json", ""},
		verisign + "wordpress.com": {200, "rdap/com-wordpress.json", ""},
	})
	ctx := context.Background()
	if _, err := c.DomainExpiry(ctx, "wordpress.com"); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(6 * 24 * time.Hour)
	if _, err := c.DomainExpiry(ctx, "wordpress.com"); err != nil {
		t.Fatal(err)
	}
	// The bootstrap is kept a day, the domain's answer a week.
	sent(t, r, bootstrap, verisign+"wordpress.com", bootstrap)
}

func TestRegistryTurnedOff(t *testing.T) {
	c, r, _ := client(nil, lookup.Registry)
	if _, err := c.DomainExpiry(context.Background(), "wordpress.com"); !errors.Is(err, lookup.ErrOff) {
		t.Fatalf("DomainExpiry() turned off = %v, want ErrOff", err)
	}
	sent(t, r)
}
