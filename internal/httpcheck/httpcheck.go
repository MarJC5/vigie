// Package httpcheck asks a site for its pages after a change, from the user's machine, as a visitor
// would, and tells whether WordPress still answers.
package httpcheck

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"
)

// Problem is why a page failed its check.
type Problem string

const (
	ProblemNone          Problem = ""
	ProblemServerError   Problem = "server-error"   // a 5xx status
	ProblemCriticalError Problem = "critical-error" // the page WordPress shows after a fatal error
	ProblemUnreachable   Problem = "unreachable"    // no answer: name, connection, TLS or timeout
	ProblemCancelled     Problem = "cancelled"
)

// Result is the check of one page.
type Result struct {
	URL     string
	Status  int
	Problem Problem
	Detail  string // the error, when unreachable
}

// Checker probes pages. A page passes unless it answers 5xx, shows WordPress's critical error
// page, or does not answer; 3xx are followed, 4xx pass (security plugins hide wp-login.php).
type Checker struct {
	Client *http.Client // a client with a timeout by default
	// PHP's OPcache can serve the code from before a change for a moment (opcache.revalidate_freq,
	// 2 s by default): Watch waits Settle, then probes every page each Interval for Window, and a
	// page must stay healthy all along.
	Settle   time.Duration
	Window   time.Duration
	Interval time.Duration
	Sleep    func(ctx context.Context, d time.Duration) error // time passing; a timer by default
}

// The markers of wp_die's page (functions.php, WordPress 7.1): the text is translated, they are not.
var markers = [][]byte{[]byte(`id="error-page"`), []byte(`class="wp-die-message"`)}

// Probe asks for one page once.
func (c Checker) Probe(ctx context.Context, url string) Result {
	r := Result{URL: url}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		r.Problem, r.Detail = ProblemUnreachable, err.Error()
		return r
	}
	req.Header.Set("Cache-Control", "no-cache")
	res, err := c.client().Do(req)
	if err != nil {
		r.Problem, r.Detail = ProblemUnreachable, err.Error()
		if ctx.Err() != nil {
			r.Problem = ProblemCancelled
		}
		return r
	}
	defer res.Body.Close()
	r.Status = res.StatusCode
	// The beginning of the page holds the markers; a whole page is not needed.
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	switch {
	case bytes.Contains(body, markers[0]) && bytes.Contains(body, markers[1]):
		r.Problem = ProblemCriticalError
	case res.StatusCode >= 500:
		r.Problem = ProblemServerError
	}
	return r
}

// Watch waits Settle, then probes the pages in rounds, Interval apart, until Window has passed: the
// first failure is returned, else a healthy result.
func (c Checker) Watch(ctx context.Context, urls []string) Result {
	if c.Settle > 0 {
		if err := c.sleep(ctx, c.Settle); err != nil {
			return Result{Problem: ProblemCancelled, Detail: err.Error()}
		}
	}
	for elapsed := time.Duration(0); ; elapsed += c.Interval {
		for _, url := range urls {
			if r := c.Probe(ctx, url); r.Problem != ProblemNone {
				return r
			}
		}
		if c.Interval <= 0 || elapsed+c.Interval >= c.Window {
			return Result{}
		}
		if err := c.sleep(ctx, c.Interval); err != nil {
			return Result{Problem: ProblemCancelled, Detail: err.Error()}
		}
	}
}

func (c Checker) client() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (c Checker) sleep(ctx context.Context, d time.Duration) error {
	if c.Sleep != nil {
		return c.Sleep(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
