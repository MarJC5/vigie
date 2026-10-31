// The docs are written in English (docs/*.md) and translated in French (docs/fr/*.md), and the
// window shows them in the user's language (ADR-0020): these tests keep both languages in step.
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

func TestEveryDocHasItsFrenchTranslation(t *testing.T) {
	en, fr := docs(t, "."), docs(t, "fr")
	if len(en) == 0 {
		t.Fatal("no doc found")
	}
	if !slices.Equal(en, fr) {
		t.Fatalf("docs/ holds %v, docs/fr/ holds %v", en, fr)
	}
}

func TestTranslationsKeepTheOutline(t *testing.T) {
	for _, name := range docs(t, ".") {
		enHeadings, enBlocks := outline(t, name)
		frHeadings, frBlocks := outline(t, filepath.Join("fr", name))
		if !slices.Equal(enHeadings, frHeadings) {
			t.Errorf("%s: heading levels %v in English, %v in French", name, enHeadings, frHeadings)
		}
		if !slices.Equal(enBlocks, frBlocks) {
			t.Errorf("%s: code blocks %q in English, %q in French", name, enBlocks, frBlocks)
		}
	}
}
