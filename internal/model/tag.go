package model

import (
	"cmp"
	"errors"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// A site's tags are free labels for finding sites across folders. They stay short enough to read
// in a row and in the site list, and few enough per site to mean something.
const (
	MaxTagLength = 40 // characters
	MaxTags      = 20 // per site
)

var (
	ErrEmptyTag   = errors.New("a tag cannot be empty")
	ErrTagTooLong = errors.New("a tag is too long")
	// ErrTagCharacter: a comma or a vertical bar would split the tag in a CSV site list, where tags
	// are separated by "|"; control characters have no place in a label.
	ErrTagCharacter = errors.New("a tag cannot contain a comma, a vertical bar or a control character")
	ErrTooManyTags  = errors.New("a site has too many tags")
)

// NormalizeTag returns a tag trimmed, its runs of spaces made one, or why it cannot be one.
func NormalizeTag(tag string) (string, error) {
	tag = strings.Join(strings.Fields(tag), " ")
	switch {
	case tag == "":
		return "", ErrEmptyTag
	case utf8.RuneCountInString(tag) > MaxTagLength:
		return "", ErrTagTooLong
	case strings.ContainsFunc(tag, func(r rune) bool { return r == ',' || r == '|' || unicode.IsControl(r) }):
		return "", ErrTagCharacter
	}
	return tag, nil
}

// TagKey is what two tags are compared by: neither case nor accents count, so "Priorité" and
// "priorite" are one tag.
func TagKey(tag string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(tag)) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// SiteTags returns the tags given to a site, normalized, each once in the order given, a tag
// already used elsewhere taking the spelling it has there.
func SiteTags(given, used []string) ([]string, error) {
	spelling := map[string]string{}
	for _, tag := range used {
		if _, ok := spelling[TagKey(tag)]; !ok {
			spelling[TagKey(tag)] = tag
		}
	}
	tags := []string{}
	seen := map[string]bool{}
	for _, tag := range given {
		tag, err := NormalizeTag(tag)
		if err != nil {
			return nil, err
		}
		key := TagKey(tag)
		if seen[key] {
			continue
		}
		seen[key] = true
		if s, ok := spelling[key]; ok {
			tag = s
		}
		tags = append(tags, tag)
	}
	if len(tags) > MaxTags {
		return nil, ErrTooManyTags
	}
	return tags, nil
}

// TagCount is a tag in use, with how many sites have it.
type TagCount struct {
	Name  string
	Sites int
}

// Tags lists the tags of the sites, by name without case or accents, each in the spelling it has.
// A tag no site has any more is gone.
func (t *Tree) Tags() []TagCount {
	byKey := map[string]*TagCount{}
	for _, s := range t.sites {
		for _, tag := range s.Tags {
			c, ok := byKey[TagKey(tag)]
			if !ok {
				c = &TagCount{Name: tag}
				byKey[TagKey(tag)] = c
			}
			c.Sites++
		}
	}
	tags := make([]TagCount, 0, len(byKey))
	for _, c := range byKey {
		tags = append(tags, *c)
	}
	slices.SortFunc(tags, func(a, b TagCount) int { return cmp.Compare(TagKey(a.Name), TagKey(b.Name)) })
	return tags
}
