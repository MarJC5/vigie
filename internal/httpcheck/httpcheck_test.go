package httpcheck_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/httpcheck"
)

// The page WordPress shows after a fatal error (wp_die in class-wp-fatal-error-handler.php).
const criticalPage = `<!DOCTYPE html><html><body id="error-page"><div class="wp-die-message">` +
	`<p>Il y a eu une erreur critique sur ce site.</p></div></body></html>`

func checker() httpcheck.Checker {
	return httpcheck.Checker{Window: 3 * time.Second, Interval: time.Second, Sleep: func(context.Context, time.Duration) error { return nil }}
}

func TestProbe(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    httpcheck.Problem
	}{
		{"a page", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("<html>ok</html>")) }, httpcheck.ProblemNone},
		// Security plugins hide wp-login.php behind a 404: not a failure.
		{"not found", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }, httpcheck.ProblemNone},
		{"a redirect to a page", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				http.Redirect(w, r, "/home/", http.StatusFound)
				return
			}
			w.Write([]byte("ok"))
		}, httpcheck.ProblemNone},
		{"server error", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) }, httpcheck.ProblemServerError},
		// The markers, not the text: the message is translated.
		{"critical error of WordPress", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(criticalPage))
		}, httpcheck.ProblemCriticalError},
		{"critical error with a 200 status", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(criticalPage)) }, httpcheck.ProblemCriticalError},
	}
	for _, tt := range tests {
		server := httptest.NewServer(tt.handler)
		got := checker().Probe(context.Background(), server.URL+"/")
		server.Close()
		if got.Problem != tt.want {
			t.Errorf("%s: %+v, want %q", tt.name, got, tt.want)
		}
	}
	got := checker().Probe(context.Background(), "http://127.0.0.1:1/")
	if got.Problem != httpcheck.ProblemUnreachable || got.Detail == "" {
		t.Errorf("closed port: %+v, want unreachable with the error", got)
	}
}

// PHP's OPcache can serve the old code for a moment after an update: a page healthy at first and
// broken a second later is a failure.
func TestWatchCatchesALateFailure(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) > 2 {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	var slept time.Duration
	c := checker()
	c.Sleep = func(_ context.Context, d time.Duration) error { slept += d; return nil }
	got := c.Watch(context.Background(), []string{server.URL + "/", server.URL + "/wp-login.php"})
	if got.Problem != httpcheck.ProblemServerError || got.URL != server.URL+"/" {
		t.Fatalf("Watch() = %+v, want the home page failing", got)
	}
	if slept != time.Second {
		t.Errorf("slept %v, want one interval before the second round", slept)
	}
}

func TestWatchHealthyForTheWholeWindow(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer server.Close()
	got := checker().Watch(context.Background(), []string{server.URL + "/", server.URL + "/wp-login.php"})
	if got.Problem != httpcheck.ProblemNone {
		t.Fatalf("Watch() = %+v", got)
	}
	// Three rounds over 3 seconds, one second apart: at 0, 1 and 2 s, then the window ends at 3 s.
	if calls.Load() != 6 {
		t.Errorf("%d requests, want 2 pages × 3 rounds", calls.Load())
	}
}

func TestWatchStopsWhenCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	c := checker()
	c.Sleep = func(ctx context.Context, _ time.Duration) error { cancel(); return ctx.Err() }
	if got := c.Watch(ctx, []string{server.URL}); got.Problem != httpcheck.ProblemCancelled {
		t.Errorf("Watch() = %+v, want cancelled", got)
	}
}
