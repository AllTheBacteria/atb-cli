package agc

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/allthebacteria/atb-cli/internal/sources"
)

func TestArchiveDir(t *testing.T) {
	if got := ArchiveDir("/data", ""); got != filepath.Join("/data", sources.AGCArchiveSubdir) {
		t.Errorf("ArchiveDir(default) = %q", got)
	}
	if got := ArchiveDir("/data", "/big/disk"); got != "/big/disk" {
		t.Errorf("ArchiveDir(override) = %q, want /big/disk", got)
	}
}

func TestDownloadArchivesCacheFirst(t *testing.T) {
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		if r.URL.Path == "/batch_a.agc" {
			io.WriteString(w, "AGC-A")
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	archiveDir := filepath.Join(t.TempDir(), "agc")
	spec := FetchSpec{ArchiveDir: archiveDir, BaseURL: srv.URL + "/", Parallel: 1}

	paths, errs := downloadArchives([]string{"batch_a", "batch_missing"}, spec)

	// batch_a downloaded; batch_missing 404 -> error.
	if _, ok := paths["batch_a"]; !ok {
		t.Errorf("batch_a should have a path, got %v / errs %v", paths, errs)
	}
	if _, ok := errs["batch_missing"]; !ok {
		t.Errorf("batch_missing should have an error, got %v", errs)
	}
	data, err := os.ReadFile(filepath.Join(archiveDir, "batch_a.agc"))
	if err != nil || string(data) != "AGC-A" {
		t.Errorf("batch_a.agc content = %q err=%v", data, err)
	}

	before := atomic.LoadInt64(&hits)
	// Second call: batch_a is cached -> no new request for it.
	if _, _ = downloadArchives([]string{"batch_a"}, spec); atomic.LoadInt64(&hits) != before {
		t.Errorf("cached archive should not be re-requested: hits went %d -> %d", before, atomic.LoadInt64(&hits))
	}
}

func TestDownloadArchivesForceRedownloads(t *testing.T) {
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		io.WriteString(w, "AGC-A")
	}))
	defer srv.Close()

	archiveDir := filepath.Join(t.TempDir(), "agc")
	spec := FetchSpec{ArchiveDir: archiveDir, BaseURL: srv.URL + "/", Parallel: 1}

	// Prime the cache so batch_a is present on disk.
	if _, errs := downloadArchives([]string{"batch_a"}, spec); len(errs) != 0 {
		t.Fatalf("priming download failed: %v", errs)
	}
	before := atomic.LoadInt64(&hits)

	// Force must bypass the cache: the archive is re-requested even though the
	// file already exists.
	spec.Force = true
	if _, errs := downloadArchives([]string{"batch_a"}, spec); len(errs) != 0 {
		t.Fatalf("forced download failed: %v", errs)
	}
	if got := atomic.LoadInt64(&hits); got <= before {
		t.Errorf("Force should re-download: hits went %d -> %d, expected an increase", before, got)
	}
}

func TestSampleFilename(t *testing.T) {
	if got := sampleFilename("ACC1", false); got != "ACC1.fa" {
		t.Errorf("sampleFilename(plain) = %q", got)
	}
	if got := sampleFilename("ACC1", true); got != "ACC1.fa.gz" {
		t.Errorf("sampleFilename(gzip) = %q", got)
	}
}

// writeFakeAGC installs a POSIX-shell `agc` stub on PATH. For each sample-name
// argument it prints ">NAME\nACGT_NAME\n"; it skips the subcommand, flag values
// (-t/-l/-g take a value), and the archive path. A sample named "BAD" makes the
// stub exit non-zero, simulating an extraction failure.
func writeFakeAGC(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub requires a POSIX shell")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"skip=0\n" +
		"for a in \"$@\"; do\n" +
		"  if [ \"$skip\" = 1 ]; then skip=0; continue; fi\n" +
		"  case \"$a\" in\n" +
		"    getset|getctg|getcol) continue ;;\n" +
		"    -t|-l|-g) skip=1; continue ;;\n" +
		"    -*) continue ;;\n" +
		"    *.agc) continue ;;\n" +
		"    BAD) echo 'no such sample: BAD' 1>&2; exit 1 ;;\n" +
		"    *) printf '>%s\\nACGT_%s\\n' \"$a\" \"$a\" ;;\n" +
		"  esac\n" +
		"done\n"
	if err := os.WriteFile(filepath.Join(dir, "agc"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake agc: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// seedArchive creates a dummy <archiveDir>/<name>.agc so downloadArchives
// treats it as a cache hit and performs no network I/O.
func seedArchive(t *testing.T, archiveDir, name string) {
	t.Helper()
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archiveDir, name+".agc"), []byte("dummy"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFetchGenomesPerSample(t *testing.T) {
	writeFakeAGC(t)
	base := t.TempDir()
	archiveDir := filepath.Join(base, "agc")
	seedArchive(t, archiveDir, "batch_a")
	outDir := filepath.Join(base, "out")

	groups := map[string][]string{"batch_a": {"ACC1", "ACC2"}}
	res, err := FetchGenomes(groups, FetchSpec{OutputDir: outDir, ArchiveDir: archiveDir, Parallel: 1})
	if err != nil {
		t.Fatalf("FetchGenomes: %v", err)
	}
	if res.Completed != 2 || res.Failed != 0 {
		t.Fatalf("result = %+v, want Completed 2 Failed 0", res)
	}
	for _, acc := range []string{"ACC1", "ACC2"} {
		data, err := os.ReadFile(filepath.Join(outDir, acc+".fa"))
		if err != nil {
			t.Fatalf("read %s.fa: %v", acc, err)
		}
		if !strings.Contains(string(data), ">"+acc) {
			t.Errorf("%s.fa = %q, want a FASTA header for %s", acc, data, acc)
		}
	}
}

func TestFetchGenomesCombinePreservesOrder(t *testing.T) {
	writeFakeAGC(t)
	base := t.TempDir()
	archiveDir := filepath.Join(base, "agc")
	seedArchive(t, archiveDir, "batch_a")

	var combined strings.Builder
	groups := map[string][]string{"batch_a": {"ACC1", "ACC2"}}
	res, err := FetchGenomes(groups, FetchSpec{Combine: true, Combined: &combined, ArchiveDir: archiveDir, Parallel: 1})
	if err != nil {
		t.Fatalf("FetchGenomes: %v", err)
	}
	if res.Completed != 2 {
		t.Errorf("Completed = %d, want 2", res.Completed)
	}
	out := combined.String()
	if i, j := strings.Index(out, ">ACC1"), strings.Index(out, ">ACC2"); i < 0 || j < 0 || i > j {
		t.Errorf("combined output should contain ACC1 before ACC2, got:\n%s", out)
	}
}

func TestFetchGenomesContinuesOnError(t *testing.T) {
	writeFakeAGC(t)
	base := t.TempDir()
	archiveDir := filepath.Join(base, "agc")
	seedArchive(t, archiveDir, "batch_a")
	outDir := filepath.Join(base, "out")

	groups := map[string][]string{"batch_a": {"ACC1", "BAD"}}
	res, err := FetchGenomes(groups, FetchSpec{OutputDir: outDir, ArchiveDir: archiveDir, Parallel: 1})
	if err != nil {
		t.Fatalf("FetchGenomes: %v", err)
	}
	if res.Completed != 1 || res.Failed != 1 {
		t.Fatalf("result = %+v, want Completed 1 Failed 1", res)
	}
	if len(res.Errors) != 1 || res.Errors[0].Accession != "BAD" {
		t.Errorf("expected one error for BAD, got %v", res.Errors)
	}
	if _, err := os.Stat(filepath.Join(outDir, "ACC1.fa")); err != nil {
		t.Errorf("ACC1.fa should exist despite BAD failing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "BAD.fa")); !os.IsNotExist(err) {
		t.Errorf("BAD.fa should have been removed after failure")
	}
}

func TestFetchGenomesCombineNoDuplicateOnPartialFailure(t *testing.T) {
	writeFakeAGC(t)
	base := t.TempDir()
	archiveDir := filepath.Join(base, "agc")
	seedArchive(t, archiveDir, "batch_a")

	var combined strings.Builder
	// The fake agc emits ACC1 then fails on BAD. Because the combined writer
	// cannot be rewound, extraction must be per-accession: ACC1 must appear
	// exactly once and only BAD must fail. The old try-batch-then-fallback
	// design wrote ACC1 twice.
	groups := map[string][]string{"batch_a": {"ACC1", "BAD"}}
	res, err := FetchGenomes(groups, FetchSpec{Combine: true, Combined: &combined, ArchiveDir: archiveDir, Parallel: 1})
	if err != nil {
		t.Fatalf("FetchGenomes: %v", err)
	}
	if res.Completed != 1 || res.Failed != 1 {
		t.Fatalf("result = %+v, want Completed 1 Failed 1", res)
	}
	if len(res.Errors) != 1 || res.Errors[0].Accession != "BAD" {
		t.Errorf("expected one error for BAD, got %v", res.Errors)
	}
	if n := strings.Count(combined.String(), ">ACC1"); n != 1 {
		t.Errorf("ACC1 should appear exactly once in combined output, got %d:\n%s", n, combined.String())
	}
}

func TestFetchGenomesDownloadFailureFailsGroup(t *testing.T) {
	writeFakeAGC(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r) // every archive download fails
	}))
	defer srv.Close()

	base := t.TempDir()
	archiveDir := filepath.Join(base, "agc")
	seedArchive(t, archiveDir, "good_batch") // cached -> no download attempted
	outDir := filepath.Join(base, "out")

	// good_batch is cached and extracts; bad_batch is absent and 404s, so every
	// accession in bad_batch's group fails with a "download bad_batch.agc:"
	// error while the healthy group still completes.
	groups := map[string][]string{
		"good_batch": {"ACC1"},
		"bad_batch":  {"ACC2", "ACC3"},
	}
	res, err := FetchGenomes(groups, FetchSpec{OutputDir: outDir, ArchiveDir: archiveDir, BaseURL: srv.URL + "/", Parallel: 1})
	if err != nil {
		t.Fatalf("FetchGenomes: %v", err)
	}
	if res.Completed != 1 || res.Failed != 2 {
		t.Fatalf("result = %+v, want Completed 1 Failed 2", res)
	}
	failed := map[string]string{}
	for _, e := range res.Errors {
		failed[e.Accession] = e.Error
	}
	for _, acc := range []string{"ACC2", "ACC3"} {
		msg, ok := failed[acc]
		if !ok {
			t.Errorf("%s should have failed", acc)
			continue
		}
		if !strings.HasPrefix(msg, "download bad_batch.agc:") {
			t.Errorf("%s error = %q, want a 'download bad_batch.agc:' prefix", acc, msg)
		}
	}
	if _, err := os.Stat(filepath.Join(outDir, "ACC1.fa")); err != nil {
		t.Errorf("ACC1.fa from the healthy group should exist: %v", err)
	}
}
