//go:build darwin && !server

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework AppKit
#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>
#include <CoreServices/CoreServices.h>
#include <stdbool.h>

static bool vigieAtLogin = false;

// The login item's launch: SMAppService passes no argument, but the Apple event that opens the
// app says it, while the app finishes launching (checked on macOS 27 for plan 08).
__attribute__((constructor)) static void vigieObserveLaunch(void) {
	[[NSNotificationCenter defaultCenter] addObserverForName:NSApplicationDidFinishLaunchingNotification object:nil queue:nil usingBlock:^(NSNotification *n) {
		NSAppleEventDescriptor *e = [[NSAppleEventManager sharedAppleEventManager] currentAppleEvent];
		vigieAtLogin = e != nil && [e eventID] == kAEOpenApplication &&
			[[e paramDescriptorForKeyword:keyAEPropData] enumCodeValue] == keyAELaunchedAsLogInItem;
	}];
}

static bool vigieLaunchedAtLogin(void) { return vigieAtLogin; }
*/
import "C"

// launchedAtLogin tells whether macOS opened Vigie as a login item; known once it finished
// launching (Wails' ApplicationStarted).
func launchedAtLogin() bool {
	return bool(C.vigieLaunchedAtLogin())
}
