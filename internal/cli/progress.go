package cli

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// Terminal is what the command line knows of where it runs (ADR-0037).
type Terminal struct {
	// Interactive: stdout and stderr are terminals, and TERM is not "dumb".
	Interactive bool
	// Width and Height are the terminal's size in columns and lines; 0 when unknown.
	Width, Height func() int
}

// spinner is the frames of the progress lines, the braille dots.
var spinner = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

// The control sequences of a VT100 terminal the block is drawn with (ADR-0037, 4b).
const (
	eraseBelow = "\x1b[J" // erase from the cursor to the end of the screen
	cursorUp   = "\x1b[%dA"
)

// item is a line of the block: something running, with its spinner, or done, with a check.
type item struct {
	text string
	done bool
}

// progress is a block of lines on stderr, redrawn in place while a command works (ADR-0037): one
// line per thing running, then a total line with the time spent; a single line when nothing runs
// apart. A nil progress draws nothing, so a command calls it either way.
type progress struct {
	out           io.Writer
	width, height func() int
	now           func() time.Time
	started       time.Time
	// more says how many lines do not fit the terminal ("… and 4 more").
	more func(n int) string

	mu    sync.Mutex
	text  string // the total line, or the single line
	items []item
	frame int
	lines int // lines of the block on screen

	stop    chan struct{}
	stopped chan struct{}
	ending  sync.Once
}

// startProgress starts the block with its first text when the command runs in a terminal and
// prints no JSON; nil otherwise. It shows at once, then about ten times a second.
func startProgress(env Env, asJSON bool, text string) *progress {
	if !env.Terminal.Interactive || asJSON {
		return nil
	}
	p := &progress{out: env.Err, width: env.Terminal.Width, height: env.Terminal.Height, now: time.Now, text: text,
		more: func(n int) string { return env.t("cli.progress.more", map[string]any{"count": n}) },
		stop: make(chan struct{}), stopped: make(chan struct{})}
	p.started = p.now()
	p.draw()
	go func() {
		defer close(p.stopped)
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-p.stop:
				return
			case <-tick.C:
				p.draw()
			}
		}
	}()
	return p
}

// set says what runs now: the total line, and the lines above it (none for a single line).
func (p *progress) set(text string, items ...item) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.text, p.items = text, items
	p.mu.Unlock()
}

// draw writes the block over the one before: back up to its first line, erase below, the lines.
func (p *progress) draw() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	turn := spinner[p.frame%len(spinner)]
	p.frame++
	var lines []string
	for _, it := range p.items {
		mark := turn
		if it.done {
			mark = '✓'
		}
		lines = append(lines, fmt.Sprintf("%c %s", mark, it.text))
	}
	total := fmt.Sprintf("%s · %s", p.text, elapsed(p.now().Sub(p.started)))
	if len(p.items) == 0 {
		total = fmt.Sprintf("%c %s", turn, total)
	}
	// Never taller than the terminal: the lines that do not fit are counted.
	if h := p.size(p.height); h > 0 && len(lines)+1 > h-1 {
		keep := max(0, h-3)
		lines = append(lines[:keep], "  "+p.more(len(lines)-keep))
	}
	lines = append(lines, total)
	for i, line := range lines {
		lines[i] = p.cut(line)
	}
	p.back()
	fmt.Fprint(p.out, eraseBelow+strings.Join(lines, "\n"))
	p.lines = len(lines)
}

// back puts the cursor at the start of the block's first line; the caller holds the lock.
func (p *progress) back() {
	if p.lines > 1 {
		fmt.Fprintf(p.out, cursorUp, p.lines-1)
	}
	fmt.Fprint(p.out, "\r")
}

// cut shortens a line to the terminal's width, or it would wrap and the next frame would not
// cover it.
func (p *progress) cut(line string) string {
	r := []rune(line)
	if w := p.size(p.width); w > 1 && len(r) > w-1 {
		return string(append(r[:w-2], '…'))
	}
	return line
}

func (p *progress) size(of func() int) int {
	if of == nil {
		return 0
	}
	return of()
}

// end stops the block and erases it, the cursor where the block began, for what follows.
func (p *progress) end() {
	if p == nil {
		return
	}
	p.ending.Do(func() {
		if p.stop != nil {
			close(p.stop)
			<-p.stopped
		}
	})
	p.mu.Lock()
	defer p.mu.Unlock()
	p.erase()
}

// above prints a line of the results while the block runs: the block is erased, the line printed,
// and the next frame draws the block under it.
func (p *progress) above(print func()) {
	if p == nil {
		print()
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.erase()
	print()
}

// erase blanks the block on screen; the caller holds the lock.
func (p *progress) erase() {
	if p.lines > 0 {
		p.back()
		fmt.Fprint(p.out, eraseBelow)
		p.lines = 0
	}
}

// bar draws how far a site's steps are: "█████░░░░ 5/9".
func bar(done, total int) string {
	const cells = 9
	filled := 0
	if total > 0 {
		filled = min(cells, done*cells/total)
	}
	return fmt.Sprintf("%s%s %d/%d", strings.Repeat("█", filled), strings.Repeat("░", cells-filled), done, total)
}

// elapsed is a time spent as the lines say it: "42 s", "1 min 05 s", "1 h 02 min".
func elapsed(d time.Duration) string {
	s := int(d.Seconds())
	switch {
	case s < 60:
		return fmt.Sprintf("%d s", s)
	case s < 3600:
		return fmt.Sprintf("%d min %02d s", s/60, s%60)
	}
	return fmt.Sprintf("%d h %02d min", s/3600, s%3600/60)
}
