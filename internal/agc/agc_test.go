package agc

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/allthebacteria/atb-cli/internal/sources"
)

func TestBinaryName(t *testing.T) {
	got := binaryName()
	want := "agc"
	if runtime.GOOS == "windows" {
		want = "agc.exe"
	}
	if got != want {
		t.Errorf("binaryName() = %q, want %q", got, want)
	}
}

func TestVersionPins(t *testing.T) {
	if sources.AGCVersion == "" {
		t.Error("sources.AGCVersion is empty")
	}
	if sources.AGCRepo != "refresh-bio/agc" {
		t.Errorf("sources.AGCRepo = %q, want refresh-bio/agc", sources.AGCRepo)
	}
}

func TestPlatformAsset(t *testing.T) {
	v := sources.AGCVersion
	cases := []struct {
		goos, goarch string
		wantAsset    string
		wantZip      bool
		wantErr      bool
	}{
		{"linux", "amd64", "agc-" + v + "-x64_linux.tar.gz", false, false},
		{"linux", "arm64", "agc-" + v + "-arm64_linux.tar.gz", false, false},
		{"darwin", "amd64", "agc-" + v + "-x64_mac.tar.gz", false, false},
		{"darwin", "arm64", "agc-" + v + "-m1_mac.tar.gz", false, false},
		{"windows", "amd64", "agc-" + v + "-x64_windows.zip", true, false},
		{"windows", "arm64", "", false, true},
		{"plan9", "amd64", "", false, true},
	}
	for _, c := range cases {
		asset, isZip, err := platformAsset(c.goos, c.goarch)
		if c.wantErr {
			if err == nil {
				t.Errorf("%s/%s: expected error, got asset %q", c.goos, c.goarch, asset)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s/%s: unexpected error: %v", c.goos, c.goarch, err)
			continue
		}
		if asset != c.wantAsset || isZip != c.wantZip {
			t.Errorf("%s/%s: got (%q, %v), want (%q, %v)",
				c.goos, c.goarch, asset, isZip, c.wantAsset, c.wantZip)
		}
	}
}

func TestFindBinaryOnPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub is a POSIX shell script named agc")
	}
	dir := t.TempDir()
	stub := filepath.Join(dir, "agc")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// FindBinary checks next to the atb binary first; in tests no agc sits
	// there, so it resolves via PATH to our stub.
	got, err := FindBinary()
	if err != nil {
		t.Fatalf("FindBinary: %v", err)
	}
	if filepath.Base(got) != "agc" {
		t.Errorf("FindBinary() = %q, want a path ending in agc", got)
	}
}

func TestFindBinaryMissing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH isolation differs on Windows")
	}
	t.Setenv("PATH", t.TempDir()) // empty dir: no agc anywhere on PATH
	if _, err := FindBinary(); err == nil {
		t.Fatal("expected error when agc is not found, got nil")
	}
}
