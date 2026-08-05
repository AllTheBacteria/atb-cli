package query

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRunFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runs.txt")
	content := "ERR1000001\n\n# a comment\nSRR2000001\n  DRR3000001  \n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write run file: %v", err)
	}

	f := Filters{RunFile: path}
	if err := f.LoadRunFile(); err != nil {
		t.Fatalf("LoadRunFile returned error: %v", err)
	}

	want := []string{"ERR1000001", "SRR2000001", "DRR3000001"}
	got := f.RunAccessions()
	if len(got) != len(want) {
		t.Fatalf("RunAccessions(): got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("RunAccessions()[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRunAccessionsUnionsFlagAndFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runs.txt")
	if err := os.WriteFile(path, []byte("SRR2000001\nERR1000001\n"), 0o644); err != nil {
		t.Fatalf("write run file: %v", err)
	}

	f := Filters{Runs: []string{"ERR1000001", "DRR3000001"}, RunFile: path}
	if err := f.LoadRunFile(); err != nil {
		t.Fatalf("LoadRunFile returned error: %v", err)
	}

	// Flag entries first, then file entries, each accession once.
	want := []string{"ERR1000001", "DRR3000001", "SRR2000001"}
	got := f.RunAccessions()
	if len(got) != len(want) {
		t.Fatalf("RunAccessions(): got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("RunAccessions()[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestHasRunFilter(t *testing.T) {
	t.Run("no runs", func(t *testing.T) {
		f := Filters{}
		if f.HasRunFilter() {
			t.Error("expected HasRunFilter() = false with no runs")
		}
	})

	t.Run("runs slice", func(t *testing.T) {
		f := Filters{Runs: []string{"ERR1000001"}}
		if !f.HasRunFilter() {
			t.Error("expected HasRunFilter() = true with a runs slice")
		}
	})

	t.Run("run file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "runs.txt")
		if err := os.WriteFile(path, []byte("ERR1000001\n"), 0o644); err != nil {
			t.Fatalf("write run file: %v", err)
		}
		f := Filters{RunFile: path}
		if err := f.LoadRunFile(); err != nil {
			t.Fatalf("LoadRunFile returned error: %v", err)
		}
		if !f.HasRunFilter() {
			t.Error("expected HasRunFilter() = true after loading a run file")
		}
	})
}

func TestLoadRunFileReportsAMissingFile(t *testing.T) {
	f := Filters{RunFile: filepath.Join(t.TempDir(), "absent.txt")}
	if err := f.LoadRunFile(); err == nil {
		t.Fatal("LoadRunFile: expected an error for a missing file")
	}
}
