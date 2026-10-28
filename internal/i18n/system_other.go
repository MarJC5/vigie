//go:build !darwin

package i18n

import "os"

// SystemLanguage returns the language of the POSIX locale variables, English otherwise.
func SystemLanguage() string {
	return LanguageFromEnv(os.Getenv)
}
