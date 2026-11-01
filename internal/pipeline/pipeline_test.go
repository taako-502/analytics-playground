package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLatestIgnoresIncompleteBatches(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LAKE_DIR", dir)
	for _, name := range []string{"20261101T010000Z", "20261101T020000Z", ".batch-incomplete"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	got, err := latest()
	if err != nil || got != "20261101T020000Z" {
		t.Fatalf("latest = %q, %v", got, err)
	}
}
func TestLatestEmpty(t *testing.T) {
	t.Setenv("LAKE_DIR", t.TempDir())
	if _, err := latest(); err == nil {
		t.Fatal("expected error for empty lake")
	}
}
func TestUnknownStage(t *testing.T) {
	if err := pipeline("invalid"); err == nil {
		t.Fatal("expected error for unknown stage")
	}
}

func TestHistoryDoesNotOverwriteCorruption(t *testing.T) {
	t.Setenv("LAKE_DIR", t.TempDir())
	p := filepath.Join(lake(), "runs.json")
	if e := os.WriteFile(p, []byte("broken"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := recordRun("all", time.Now(), nil); e == nil {
		t.Fatal("corruption must be reported")
	}
	b, _ := os.ReadFile(p)
	if string(b) != "broken" {
		t.Fatal("overwrote history")
	}
}
func TestHistoryRoundTrip(t *testing.T) {
	t.Setenv("LAKE_DIR", t.TempDir())
	for range 2 {
		if e := recordRun("all", time.Now(), nil); e != nil {
			t.Fatal(e)
		}
	}
	b, e := os.ReadFile(filepath.Join(lake(), "runs.json"))
	if e != nil {
		t.Fatal(e)
	}
	var entries []any
	if e = json.Unmarshal(b, &entries); e != nil || len(entries) != 2 {
		t.Fatal(string(b), e)
	}
}
