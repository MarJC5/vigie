//go:build darwin && !server

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation
#import <Foundation/Foundation.h>
#include <stdlib.h>

// vigieTrash moves a file or folder to the Trash with NSFileManager trashItemAtURL (macOS 10.8 and
// later); NULL when done, else the error's description, to free.
static char *vigieTrash(const char *path) {
	@autoreleasepool {
		NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
		NSError *error = nil;
		if ([[NSFileManager defaultManager] trashItemAtURL:url resultingItemURL:nil error:&error]) {
			return NULL;
		}
		return strdup([[error localizedDescription] UTF8String]);
	}
}
*/
import "C"

import (
	"errors"
	"os/exec"
	"unsafe"

	"github.com/MarJC5/vigie/internal/trash"
)

// toTrash moves a copy to the Trash of macOS (ADR-0041); a disk without one (a network share) is
// trash.ErrNoTrash, and the window offers to delete it for good.
func toTrash(path string) error {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	if msg := C.vigieTrash(p); msg != nil {
		defer C.free(unsafe.Pointer(msg))
		return errors.Join(trash.ErrNoTrash, errors.New(C.GoString(msg)))
	}
	return nil
}

// reveal shows a copy's folder in the Finder: open -R "Reveals the file(s) in the Finder instead of
// opening them" (man open).
func reveal(path string) error {
	return exec.Command("/usr/bin/open", "-R", path).Run()
}
