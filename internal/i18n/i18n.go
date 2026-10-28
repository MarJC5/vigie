// Package i18n reads the translation catalogs shared with the interface (i18next JSON: nested keys,
// {{name}} placeholders, _one, _many and _other plural suffixes) for the texts Go shows itself.
package i18n

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"
	"strings"
)

// Languages lists the supported languages; the first one is the fallback.
var Languages = []string{"en", "fr"}

// Catalogs holds every language's texts, flattened to dotted keys.
type Catalogs struct {
	texts map[string]map[string]string
}

// Load reads <language>.json for every supported language.
func Load(files fs.FS) (*Catalogs, error) {
	c := &Catalogs{texts: map[string]map[string]string{}}
	for _, lang := range Languages {
		data, err := fs.ReadFile(files, lang+".json")
		if err != nil {
			return nil, err
		}
		var tree map[string]any
		if err := json.Unmarshal(data, &tree); err != nil {
			return nil, fmt.Errorf("%s.json: %w", lang, err)
		}
		flat := map[string]string{}
		if err := flatten("", tree, flat); err != nil {
			return nil, fmt.Errorf("%s.json: %w", lang, err)
		}
		c.texts[lang] = flat
	}
	return c, nil
}

func flatten(prefix string, tree map[string]any, out map[string]string) error {
	for k, v := range tree {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch v := v.(type) {
		case string:
			out[key] = v
		case map[string]any:
			if err := flatten(key, v, out); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s: only strings and objects are allowed", key)
		}
	}
	return nil
}

// Keys returns the keys of a language, sorted.
func (c *Catalogs) Keys(lang string) []string {
	keys := make([]string, 0, len(c.texts[lang]))
	for k := range c.texts[lang] {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// T returns the text of key in lang, with its placeholders filled from vars. A "count" variable
// selects the plural form. Unsupported languages fall back to English, missing keys to the key.
func (c *Catalogs) T(lang, key string, vars map[string]any) string {
	if _, ok := c.texts[lang]; !ok {
		lang = Languages[0]
	}
	for _, l := range []string{lang, Languages[0]} {
		if text, ok := c.lookup(l, key, vars); ok {
			return interpolate(text, vars)
		}
	}
	return key
}

func (c *Catalogs) lookup(lang, key string, vars map[string]any) (string, bool) {
	texts := c.texts[lang]
	if count, ok := vars["count"].(int); ok {
		for _, form := range []string{PluralCategory(lang, count), "other"} {
			if text, ok := texts[key+"_"+form]; ok {
				return text, true
			}
		}
	}
	text, ok := texts[key]
	return text, ok
}

func interpolate(text string, vars map[string]any) string {
	for name, value := range vars {
		text = strings.ReplaceAll(text, "{{"+name+"}}", fmt.Sprint(value))
	}
	return text
}

// PluralCategory returns the CLDR cardinal category of an integer count, following the current
// CLDR rules for the supported languages (the same rules as the interface's Intl.PluralRules):
// English "one" for 1; French "one" for 0 and 1, "many" for exact millions; otherwise "other".
func PluralCategory(lang string, count int) string {
	n := count
	if n < 0 {
		n = -n
	}
	switch lang {
	case "fr":
		switch {
		case n <= 1:
			return "one"
		case n%1000000 == 0:
			return "many"
		}
	default:
		if n == 1 {
			return "one"
		}
	}
	return "other"
}

// Supported returns the supported language of a tag such as "fr-CH" or "fr_CH.UTF-8", or "".
func Supported(tag string) string {
	base, _, _ := strings.Cut(strings.ToLower(tag), "_")
	base, _, _ = strings.Cut(base, "-")
	base, _, _ = strings.Cut(base, ".")
	if slices.Contains(Languages, base) {
		return base
	}
	return ""
}

// LanguageFromEnv picks the language from the POSIX locale variables in gettext's order: LANGUAGE
// (a priority list), then LC_ALL, LC_MESSAGES and LANG; empty variables are ignored. English when
// nothing supported is found.
func LanguageFromEnv(getenv func(string) string) string {
	for _, tag := range strings.Split(getenv("LANGUAGE"), ":") {
		if lang := Supported(tag); lang != "" {
			return lang
		}
	}
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := getenv(name); value != "" {
			if lang := Supported(value); lang != "" {
				return lang
			}
			return Languages[0]
		}
	}
	return Languages[0]
}
