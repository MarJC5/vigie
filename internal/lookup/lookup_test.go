package lookup_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/lookup"
	"github.com/MarJC5/vigie/internal/store"
)

// replay answers requests from files of testdata, by URL, and records every request made.
type replay struct {
	mu       sync.Mutex
	answers  map[string]answer
	requests []*http.Request
	down     bool // every request fails, as without network
}

type answer struct {
	status      int
	file        string // under testdata
	body        string // when there is no file
	contentType string
	location    string // a redirect's target
}

func (r *replay) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	r.requests = append(r.requests, req)
	r.mu.Unlock()
	if r.down {
		return nil, errors.New("network is unreachable")
	}
	a, ok := r.answers[req.URL.String()]
	if !ok {
		return &http.Response{StatusCode: 599, Body: io.NopCloser(bytes.NewReader(nil)), Request: req}, nil
	}
	body := []byte(a.body)
	if a.file != "" {
		var err error
		if body, err = os.ReadFile(filepath.Join("testdata", a.file)); err != nil {
			return nil, err
		}
	}
	header := http.Header{}
	if a.contentType != "" {
		header.Set("Content-Type", a.contentType)
	}
	if a.location != "" {
		header.Set("Location", a.location)
	}
	return &http.Response{StatusCode: a.status, Header: header, Body: io.NopCloser(bytes.NewReader(body)), Request: req}, nil
}

// memoryCache is store.DB's cache, in memory.
type memoryCache struct {
	mu      sync.Mutex
	answers map[string]store.LookupAnswer
}

func (m *memoryCache) Lookup(_ context.Context, source, key string) (store.LookupAnswer, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.answers[source+" "+key]
	return a, ok, nil
}

func (m *memoryCache) PutLookup(_ context.Context, source, key string, a store.LookupAnswer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.answers[source+" "+key] = a
	return nil
}

const userAgent = "Vigie/0.1.0 (+https://github.com/MarJC5/vigie)"

// client returns a client over the replayed answers, with every destination allowed but those
// in off, and a clock the test moves.
func client(answers map[string]answer, off ...lookup.Destination) (lookup.Client, *replay, *time.Time) {
	r := &replay{answers: answers}
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	c := lookup.Client{
		HTTP:      &http.Client{Transport: r},
		UserAgent: userAgent,
		Cache:     &memoryCache{answers: map[string]store.LookupAnswer{}},
		Now:       func() time.Time { return now },
		Allowed: func(d lookup.Destination) bool {
			for _, o := range off {
				if o == d {
					return false
				}
			}
			return true
		},
	}
	return c, r, &now
}

// sent checks that exactly these URLs were asked, with Vigie's identification and nothing else.
func sent(t *testing.T, r *replay, urls ...string) {
	t.Helper()
	if len(r.requests) != len(urls) {
		t.Fatalf("%d requests, want %d (%v)", len(r.requests), len(urls), urls)
	}
	for i, req := range r.requests {
		if req.Method != http.MethodGet || req.URL.String() != urls[i] {
			t.Errorf("request %d = %s %s, want GET %s", i, req.Method, req.URL, urls[i])
		}
		if got := req.Header.Get("User-Agent"); got != userAgent {
			t.Errorf("User-Agent = %q, want %q", got, userAgent)
		}
		if req.Header.Get("Cookie") != "" || req.Header.Get("Referer") != "" || req.Body != nil && req.ContentLength > 0 {
			t.Errorf("request %d carries more than its URL and identification", i)
		}
	}
}
