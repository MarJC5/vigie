//go:build !darwin || server

package main

// launchedAtLogin: elsewhere, the autostart entry passes --at-login.
func launchedAtLogin() bool { return false }
