package app

import "errors"

// Autostart is the system's login items, as Wails gives them (AutostartManager).
type Autostart interface {
	Enable() error
	Disable() error
	IsEnabled() (bool, error)
}

// LoginItem starts Vigie at login, with its window hidden. Its state is the system's: the user can
// turn it off in the system's settings, so Vigie never stores it.
type LoginItem struct {
	system      Autostart
	unsupported error // what the system answers where it has no login items
}

// LoginItemState is what the window shows: whether the switch is offered, and its state.
type LoginItemState struct {
	Supported bool `json:"supported"`
	On        bool `json:"on"`
}

// NewLoginItem returns the service; unsupported is the error of a system without login items.
func NewLoginItem(system Autostart, unsupported error) *LoginItem {
	return &LoginItem{system: system, unsupported: unsupported}
}

// State reads the login item from the system.
func (l *LoginItem) State() (LoginItemState, error) {
	on, err := l.system.IsEnabled()
	if errors.Is(err, l.unsupported) {
		return LoginItemState{}, nil
	}
	if err != nil {
		return LoginItemState{}, err
	}
	return LoginItemState{Supported: true, On: on}, nil
}

// Set adds Vigie to the login items, or removes it.
func (l *LoginItem) Set(on bool) error {
	if on {
		return l.system.Enable()
	}
	return l.system.Disable()
}
