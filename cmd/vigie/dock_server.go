//go:build server

package main

// The server build of the end-to-end tests has no Dock.
type noDock struct{}

func (noDock) ShowAppIcon() {}
func (noDock) HideAppIcon() {}

func newDock() appDock { return noDock{} }
