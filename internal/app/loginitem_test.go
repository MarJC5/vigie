package app_test

import (
	"errors"
	"testing"

	"github.com/MarJC5/vigie/internal/app"
)

// system is a fake of the system's login items.
type system struct {
	on   bool
	fail error
}

func (s *system) Enable() error {
	if s.fail != nil {
		return s.fail
	}
	s.on = true
	return nil
}

func (s *system) Disable() error {
	if s.fail != nil {
		return s.fail
	}
	s.on = false
	return nil
}

func (s *system) IsEnabled() (bool, error) { return s.on, s.fail }

var errUnsupported = errors.New("not supported")

// The login item is the system's: read from it, never stored by Vigie.
func TestTheLoginItemIsTheSystems(t *testing.T) {
	sys := &system{}
	item := app.NewLoginItem(sys, errUnsupported)
	if got, err := item.State(); err != nil || !got.Supported || got.On {
		t.Fatalf("State() = %+v, %v; want supported and off", got, err)
	}
	if err := item.Set(true); err != nil || !sys.on {
		t.Fatalf("Set(true): %v, system on = %v", err, sys.on)
	}
	sys.on = false // turned off in the system's settings
	if got, _ := item.State(); got.On {
		t.Fatal("State() kept a value of its own: want the system's")
	}
}

// Where the system has no login items (the server build), the switch is not offered.
func TestNoLoginItemWhereUnsupported(t *testing.T) {
	item := app.NewLoginItem(&system{fail: errUnsupported}, errUnsupported)
	if got, err := item.State(); err != nil || got.Supported {
		t.Fatalf("State() = %+v, %v; want not supported, without an error", got, err)
	}
}
