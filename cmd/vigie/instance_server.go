//go:build server

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// The end-to-end tests run several servers at once, each on its own data: no single instance.
func singleInstance(func()) *application.SingleInstanceOptions { return nil }
