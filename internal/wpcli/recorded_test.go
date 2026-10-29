package wpcli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// recorded is one WP-CLI call captured from the container bench (make record).
type recorded struct {
	Args   []string `json:"args"`
	Exit   int      `json:"exit"`
	Stdout string   `json:"stdout"`
	Stderr string   `json:"stderr"`
}

func load(t testing.TB, fixture, name string) recorded {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "recorded", fixture, name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var r recorded
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	return r
}
