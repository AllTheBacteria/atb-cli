package agc

import (
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
