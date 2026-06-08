package agc

import (
	"slices"
	"strings"
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

func TestParseList(t *testing.T) {
	in := "sample_a\n\n  sample_b  \n\nsample_c\n"
	got := parseList(strings.NewReader(in))
	want := []string{"sample_a", "sample_b", "sample_c"}
	if !slices.Equal(got, want) {
		t.Errorf("parseList = %v, want %v", got, want)
	}
}

func TestParseListEmpty(t *testing.T) {
	if got := parseList(strings.NewReader("\n  \n")); len(got) != 0 {
		t.Errorf("parseList(blank) = %v, want empty", got)
	}
}

func TestParseListCRLF(t *testing.T) {
	got := parseList(strings.NewReader("sample_a\r\n\r\n  sample_b  \r\nsample_c\r\n"))
	want := []string{"sample_a", "sample_b", "sample_c"}
	if !slices.Equal(got, want) {
		t.Errorf("parseList(crlf) = %v, want %v", got, want)
	}
}
