//go:build !server

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// singleInstance keeps one window on the database (ADR-0036): another launch brings the open
// window forward and exits. The second launch is told off the main thread, as opening waits for it.
func singleInstance(open func()) *application.SingleInstanceOptions {
	return &application.SingleInstanceOptions{
		UniqueID:               "io.github.marjc5.vigie",
		OnSecondInstanceLaunch: func(application.SecondInstanceData) { go open() },
	}
}
