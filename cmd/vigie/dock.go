//go:build !server

package main

import "github.com/wailsapp/wails/v3/pkg/services/dock"

func newDock() appDock { return dock.New() }
