package model_test

import (
	"errors"
	"testing"

	"github.com/MarJC5/vigie/internal/model"
)

func TestNewIDIsVersion7AndIncreasing(t *testing.T) {
	a, b := model.NewID(), model.NewID()
	if a.String()[14] != '7' {
		t.Fatalf("NewID() = %s, want a version 7 UUID", a)
	}
	if a.Compare(b) >= 0 {
		t.Fatalf("NewID() not increasing: %s then %s", a, b)
	}
}

func TestValidateName(t *testing.T) {
	tests := []struct {
		name string
		want error
	}{
		{"Clients", nil},
		{"misits.ch", nil},
		{"  Padded  ", nil},
		{"", model.ErrEmptyName},
		{"   ", model.ErrEmptyName},
		{"Clients/Infomaniak", model.ErrSlashInName},
	}
	for _, tt := range tests {
		if got := model.ValidateName(tt.name); !errors.Is(got, tt.want) {
			t.Errorf("ValidateName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestColorsFollowFileZillaOrder(t *testing.T) {
	want := []string{"none", "red", "green", "blue", "yellow", "cyan", "magenta", "orange"}
	if len(model.Colors()) != len(want) {
		t.Fatalf("Colors() has %d values, want %d", len(model.Colors()), len(want))
	}
	for i, c := range model.Colors() {
		if int(c) != i || c.String() != want[i] {
			t.Errorf("Colors()[%d] = %d %q, want %d %q", i, c, c, i, want[i])
		}
	}
}

func TestParseColor(t *testing.T) {
	if c, err := model.ParseColor("magenta"); err != nil || c != model.ColorMagenta {
		t.Fatalf("ParseColor(magenta) = %v, %v", c, err)
	}
	if _, err := model.ParseColor("purple"); !errors.Is(err, model.ErrUnknownColor) {
		t.Fatalf("ParseColor(purple) error = %v, want ErrUnknownColor", err)
	}
}

func TestHostConnection(t *testing.T) {
	tests := []struct {
		name string
		host model.Host
		want error
	}{
		{"alias only", model.Host{Name: "h", Alias: "infomaniak"}, nil},
		{"hostname and user", model.Host{Name: "h", Hostname: "example.org", User: "deploy"}, nil},
		{"nothing", model.Host{Name: "h"}, model.ErrNoConnection},
		{"alias and hostname", model.Host{Name: "h", Alias: "a", Hostname: "example.org", User: "u"}, model.ErrNoConnection},
		{"hostname without user", model.Host{Name: "h", Hostname: "example.org"}, model.ErrNoUser},
		{"port too high", model.Host{Name: "h", Hostname: "example.org", User: "u", Port: 70000}, model.ErrInvalidPort},
		{"bad name", model.Host{Name: "a/b", Alias: "a"}, model.ErrSlashInName},
	}
	for _, tt := range tests {
		if got := tt.host.Validate(); !errors.Is(got, tt.want) {
			t.Errorf("%s: Validate() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestHostPortDefaultsTo22(t *testing.T) {
	if got := (model.Host{}).EffectivePort(); got != 22 {
		t.Fatalf("EffectivePort() = %d, want 22", got)
	}
	if got := (model.Host{Port: 18765}).EffectivePort(); got != 18765 {
		t.Fatalf("EffectivePort() = %d, want 18765", got)
	}
}

func TestColorValid(t *testing.T) {
	for _, c := range model.Colors() {
		if err := c.Validate(); err != nil {
			t.Errorf("%s: %v", c, err)
		}
	}
	for _, c := range []model.Color{-1, 8} {
		if err := c.Validate(); !errors.Is(err, model.ErrUnknownColor) {
			t.Errorf("Color(%d).Validate() = %v, want ErrUnknownColor", int(c), err)
		}
	}
}

// A host's account is its alias, else user@hostname:port.
func TestHostAccount(t *testing.T) {
	if got := (model.Host{Alias: "vps"}).Account(); got != "vps" {
		t.Errorf("alias: %q", got)
	}
	if got := (model.Host{User: "u12ab", Hostname: "ssh.example-host.ch"}).Account(); got != "u12ab@ssh.example-host.ch:22" {
		t.Errorf("host: %q", got)
	}
}
