package agc

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

func TestGetContigsStreams(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub is a POSIX shell script")
	}
	dir := t.TempDir()
	stub := "#!/bin/sh\nprintf '>ctg1\\nACGTACGT\\n'\n"
	if err := os.WriteFile(filepath.Join(dir, "agc"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var buf bytes.Buffer
	if err := GetContigs("x.agc", []string{"ctg1"}, &buf, Options{Threads: 1}); err != nil {
		t.Fatalf("GetContigs: %v", err)
	}
	if !strings.Contains(buf.String(), ">ctg1") || !strings.Contains(buf.String(), "ACGTACGT") {
		t.Errorf("streamed output = %q, want a FASTA containing >ctg1 / ACGTACGT", buf.String())
	}
}

func TestGetContigsSurfacesError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub is a POSIX shell script")
	}
	dir := t.TempDir()
	stub := "#!/bin/sh\necho 'boom: bad archive' 1>&2\nexit 3\n"
	if err := os.WriteFile(filepath.Join(dir, "agc"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var buf bytes.Buffer
	err := GetContigs("x.agc", []string{"ctg1"}, &buf, Options{Threads: 1})
	if err == nil {
		t.Fatal("expected error on non-zero agc exit, got nil")
	}
	if !strings.Contains(err.Error(), "boom: bad archive") {
		t.Errorf("error = %v, want it to include agc stderr", err)
	}
}

// TestIntegrationRoundTrip builds a tiny archive with the real agc binary, then
// asserts list + extract round-trip the sequence. Skipped when agc is absent.
func TestIntegrationRoundTrip(t *testing.T) {
	if _, err := FindBinary(); err != nil {
		t.Skip("agc not installed")
	}
	dir := t.TempDir()

	// ~1 kb single-contig FASTA — long enough for agc's default k-mer/segment.
	var seq strings.Builder
	const bases = "ACGT"
	for i := 0; i < 1000; i++ {
		seq.WriteByte(bases[i%4])
	}
	faPath := filepath.Join(dir, "sample1.fa")
	if err := os.WriteFile(faPath, []byte(">ctg1\n"+seq.String()+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	bin, _ := FindBinary()
	archive := filepath.Join(dir, "test.agc")
	if out, err := exec.Command(bin, "create", "-o", archive, faPath).CombinedOutput(); err != nil {
		t.Fatalf("agc create: %v\n%s", err, out)
	}

	samples, err := ListSamples(archive)
	if err != nil {
		t.Fatalf("ListSamples: %v", err)
	}
	if len(samples) == 0 {
		t.Fatal("ListSamples returned no samples")
	}

	contigs, err := ListContigs(archive, samples[0])
	if err != nil {
		t.Fatalf("ListContigs: %v", err)
	}
	if len(contigs) == 0 {
		t.Fatal("ListContigs returned no contigs")
	}

	var buf bytes.Buffer
	if err := GetContigs(archive, []string{contigs[0]}, &buf, Options{Threads: 1}); err != nil {
		t.Fatalf("GetContigs: %v", err)
	}
	if got := stripFASTA(buf.String()); got != seq.String() {
		t.Errorf("round-trip mismatch: got %d bases, want %d", len(got), seq.Len())
	}
}

// stripFASTA returns the concatenated sequence of a FASTA string, dropping
// header lines and line wrapping.
func stripFASTA(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, ">") {
			continue
		}
		b.WriteString(strings.TrimSpace(line))
	}
	return b.String()
}
