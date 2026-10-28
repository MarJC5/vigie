package i18n

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation
#import <Foundation/Foundation.h>
#include <stdlib.h>

// The user's ordered list of preferred languages, as one comma-separated string to free.
static char *preferredLanguages(void) {
	@autoreleasepool {
		NSString *joined = [[NSLocale preferredLanguages] componentsJoinedByString:@","];
		return strdup(joined.UTF8String);
	}
}
*/
import "C"

import (
	"strings"
	"unsafe"
)

// SystemLanguage returns the first supported language of the user's macOS preferred languages
// (NSLocale.preferredLanguages), English otherwise.
func SystemLanguage() string {
	list := C.preferredLanguages()
	defer C.free(unsafe.Pointer(list))
	for _, tag := range strings.Split(C.GoString(list), ",") {
		if lang := Supported(tag); lang != "" {
			return lang
		}
	}
	return Languages[0]
}
