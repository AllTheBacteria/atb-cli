package agc

import (
	"slices"
	"testing"
)

func TestBuildGetArgs(t *testing.T) {
	// Streaming supported, every option set.
	got := buildGetArgs(Options{Threads: 4, LineLength: 60, GzipLevel: 6, Streaming: true}, true)
	want := []string{"-t", "4", "-l", "60", "-g", "6", "-s"}
	if !slices.Equal(got, want) {
		t.Errorf("buildGetArgs(full) = %v, want %v", got, want)
	}

	// getcol path: streaming unsupported, so -s is dropped even when requested.
	got = buildGetArgs(Options{Threads: 2, Streaming: true}, false)
	want = []string{"-t", "2"}
	if !slices.Equal(got, want) {
		t.Errorf("buildGetArgs(getcol) = %v, want %v", got, want)
	}

	// Zero line length and gzip level are omitted; only -t remains.
	got = buildGetArgs(Options{Threads: 1}, true)
	want = []string{"-t", "1"}
	if !slices.Equal(got, want) {
		t.Errorf("buildGetArgs(defaults) = %v, want %v", got, want)
	}

	// LineLength set but GzipLevel zero: -l present, -g omitted (guards are independent).
	got = buildGetArgs(Options{Threads: 1, LineLength: 70}, true)
	want = []string{"-t", "1", "-l", "70"}
	if !slices.Equal(got, want) {
		t.Errorf("buildGetArgs(line-only) = %v, want %v", got, want)
	}

	// GzipLevel set but LineLength zero: -g present, -l omitted.
	got = buildGetArgs(Options{Threads: 1, GzipLevel: 6}, true)
	want = []string{"-t", "1", "-g", "6"}
	if !slices.Equal(got, want) {
		t.Errorf("buildGetArgs(gzip-only) = %v, want %v", got, want)
	}
}
