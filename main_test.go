package main

import (
	"os"
	"path/filepath"
	"testing"
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
