// The docs are written in English (docs/*.md) and translated in French (docs/fr/*.md), as are the
// short topics of the window's help (docs/help/, ADR-0021): these tests keep both languages in step.
package docs_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// outline is what a translation keeps of its source: the level of each heading, and the language
// of each code block, in order.
func outline(t *testing.T, path string) (headings []int, blocks []string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fenced := false
	for line := range strings.SplitSeq(string(data), "\n") {
		if lang, ok := strings.CutPrefix(line, "```"); ok {
			if !fenced {
				blocks = append(blocks, lang)
			}
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if level := len(line) - len(strings.TrimLeft(line, "#")); level > 0 && strings.HasPrefix(line[level:], " ") {
			headings = append(headings, level)
		}
	}
	if fenced {
		t.Errorf("%s: a code block is not closed", path)
	}
	return headings, blocks
}

func docs(t *testing.T, dir string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(paths))
	for i, p := range paths {
		names[i] = filepath.Base(p)
	}
	return names
}

// The docs, then the short topics of the window's help (ADR-0021), each with its translation.
var sets = []struct{ en, fr string }{{".", "fr"}, {"help", filepath.Join("fr", "help")}}

func TestEveryDocHasItsFrenchTranslation(t *testing.T) {
	for _, set := range sets {
		en, fr := docs(t, set.en), docs(t, set.fr)
		if len(en) == 0 {
			t.Fatalf("no doc found in %s", set.en)
		}
		if !slices.Equal(en, fr) {
			t.Errorf("%s holds %v, %s holds %v", set.en, en, set.fr, fr)
		}
	}
}

func TestTranslationsKeepTheOutline(t *testing.T) {
	for _, set := range sets {
		for _, name := range docs(t, set.en) {
			enHeadings, enBlocks := outline(t, filepath.Join(set.en, name))
			frHeadings, frBlocks := outline(t, filepath.Join(set.fr, name))
			if !slices.Equal(enHeadings, frHeadings) {
				t.Errorf("%s: heading levels %v in English, %v in French", filepath.Join(set.en, name), enHeadings, frHeadings)
			}
			if !slices.Equal(enBlocks, frBlocks) {
				t.Errorf("%s: code blocks %q in English, %q in French", filepath.Join(set.en, name), enBlocks, frBlocks)
			}
		}
	}
}

// A topic of the help is read in a minute: 300 words at most, in either language.
func TestHelpTopicsAreShort(t *testing.T) {
	for _, dir := range []string{"help", filepath.Join("fr", "help")} {
		for _, name := range docs(t, dir) {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			if n := len(strings.Fields(string(data))); n > 300 {
				t.Errorf("%s: %d words, 300 at most", filepath.Join(dir, name), n)
			}
		}
	}
}
