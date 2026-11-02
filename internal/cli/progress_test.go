package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

// fakeProgress is a block drawn in a buffer, with a clock and a terminal of the size given.
func fakeProgress(out *bytes.Buffer, now *time.Time, width, height int) *progress {
	start := *now
	return &progress{out: out, width: func() int { return width }, height: func() int { return height },
		now: func() time.Time { return *now }, started: start,
		more: func(n int) string { return fmt.Sprintf("… and %d more", n) }}
}

// A single line: spinner, text, time spent, drawn over itself, erased at the end (ADR-0037).
func TestTheSingleLine(t *testing.T) {
	var out bytes.Buffer
	now := time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC)
	p := fakeProgress(&out, &now, 80, 24)
	p.set("Looking for WordPress installs on bench")
	now = now.Add(8 * time.Second)
	p.draw()
	if got, want := out.String(), "\r\x1b[J⠋ Looking for WordPress installs on bench · 8 s"; got != want {
		t.Fatalf("frame = %q, want %q", got, want)
	}
	out.Reset()
	p.end()
	if got, want := out.String(), "\r\x1b[J"; got != want {
		t.Fatalf("end = %q, want %q", got, want)
	}
}

// A block: a line per site running with its spinner, the sites done with a check, the total with
// the time spent; the next frame goes back up over it (ADR-0037, 2b and 4b).
func TestTheBlock(t *testing.T) {
	var out bytes.Buffer
	now := time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC)
	p := fakeProgress(&out, &now, 80, 24)
	p.set("Scanning: 1 of 3 sites · 2 running",
		item{text: "coolectif.ch  " + bar(5, 9)}, item{text: "misits.ch     " + bar(2, 9)}, item{text: "jean-christio.ch  done", done: true})
	now = now.Add(65 * time.Second)
	p.draw()
	want := "\r\x1b[J" + strings.Join([]string{
		"⠋ coolectif.ch  █████░░░░ 5/9",
		"⠋ misits.ch     ██░░░░░░░ 2/9",
		"✓ jean-christio.ch  done",
		"Scanning: 1 of 3 sites · 2 running · 1 min 05 s",
	}, "\n")
	if got := out.String(); got != want {
		t.Fatalf("first frame = %q\nwant %q", got, want)
	}
	out.Reset()
	p.draw()
	if got := out.String(); !strings.HasPrefix(got, "\x1b[3A\r\x1b[J⠙ coolectif.ch") {
		t.Fatalf("second frame = %q, want it back up 3 lines", got)
	}
	out.Reset()
	p.end()
	if got, want := out.String(), "\x1b[3A\r\x1b[J"; got != want {
		t.Fatalf("end = %q, want %q", got, want)
	}
}

// Never taller than the terminal: the lines that do not fit are counted; each cut to its width.
func TestTheBlockFitsTheTerminal(t *testing.T) {
	var out bytes.Buffer
	now := time.Now()
	p := fakeProgress(&out, &now, 20, 6)
	var items []item
	for range 8 {
		items = append(items, item{text: "a-long-site-name.example.ch"})
	}
	p.set("Scanning", items...)
	p.draw()
	lines := strings.Split(strings.TrimPrefix(out.String(), "\r\x1b[J"), "\n")
	if len(lines) != 5 || !strings.Contains(lines[3], "and 5 more") {
		t.Fatalf("lines = %q, want 3 sites, the count of the 5 others, the total", lines)
	}
	for _, l := range lines {
		if n := len([]rune(l)); n > 19 {
			t.Errorf("line %q is %d wide, want 19 at most", l, n)
		}
	}
}

// Nothing at all outside a terminal or with --json.
func TestNoProgressOutsideATerminal(t *testing.T) {
	for _, c := range []struct {
		name   string
		term   Terminal
		asJSON bool
	}{
		{"not a terminal", Terminal{}, false},
		{"--json", Terminal{Interactive: true}, true},
	} {
		var out bytes.Buffer
		p := startProgress(Env{Err: &out, Terminal: c.term}, c.asJSON, "Scanning")
		p.set("Scanning")
		p.end()
		if p != nil || out.Len() != 0 {
			t.Errorf("%s: drew %q", c.name, out.String())
		}
	}
}

func TestBar(t *testing.T) {
	for _, c := range []struct {
		done, total int
		want        string
	}{{0, 9, "░░░░░░░░░ 0/9"}, {5, 9, "█████░░░░ 5/9"}, {9, 9, "█████████ 9/9"}, {0, 0, "░░░░░░░░░ 0/0"}} {
		if got := bar(c.done, c.total); got != c.want {
			t.Errorf("bar(%d, %d) = %q, want %q", c.done, c.total, got, c.want)
		}
	}
}
