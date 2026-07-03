package agc

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/allthebacteria/atb-cli/internal/download"
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

func TestBuildDownloadTasksUsesRefs(t *testing.T) {
	archives := []string{"Acinetobacter_baylyi_global_ordered_0001", "legacy_batch"}
	spec := FetchSpec{
		BaseURL: "https://cesgo.example/agc/",
		Refs: map[string]ArchiveRef{
			"Acinetobacter_baylyi_global_ordered_0001": {
				Name: "Acinetobacter_baylyi_global_ordered_0001",
				URL:  "https://osf.io/download/aaa/",
				MD5:  "md5aaa",
			},
		},
	}

	tasks, urlToArchive := buildDownloadTasks(archives, spec)
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(tasks))
	}

	var osfTask, legacyTask download.FileTask
	for _, tk := range tasks {
		switch tk.Filename {
		case "Acinetobacter_baylyi_global_ordered_0001.agc":
			osfTask = tk
		case "legacy_batch.agc":
			legacyTask = tk
		default:
			t.Errorf("unexpected task filename %q", tk.Filename)
		}
	}

	// Archive present in Refs: opaque OSF URL + md5 carried through for
	// download and verification (cannot be built from the name).
	if osfTask.URL != "https://osf.io/download/aaa/" {
		t.Errorf("osf task URL = %q, want the ref URL", osfTask.URL)
	}
	if osfTask.MD5 != "md5aaa" {
		t.Errorf("osf task MD5 = %q, want md5aaa", osfTask.MD5)
	}

	// Archive absent from Refs: falls back to the constructed base URL, no md5.
	if legacyTask.URL != "https://cesgo.example/agc/legacy_batch.agc" {
		t.Errorf("legacy task URL = %q, want constructed fallback", legacyTask.URL)
	}
	if legacyTask.MD5 != "" {
		t.Errorf("legacy task MD5 = %q, want empty (no ref)", legacyTask.MD5)
	}

	// The url->archive map must let a download error be attributed back to the
	// archive name regardless of which URL form was used.
	if urlToArchive[osfTask.URL] != "Acinetobacter_baylyi_global_ordered_0001" {
		t.Errorf("urlToArchive[%q] = %q, want the OSF archive name", osfTask.URL, urlToArchive[osfTask.URL])
	}
	if urlToArchive[legacyTask.URL] != "legacy_batch" {
		t.Errorf("urlToArchive[%q] = %q, want legacy_batch", legacyTask.URL, urlToArchive[legacyTask.URL])
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

// writeFakeAGCCollection installs an `agc` stub on PATH that supports getcol:
// `agc getcol <archive>` prints two whole-archive FASTA records. If the archive
// path contains "FAILCOL" it exits non-zero, simulating an extraction failure.
func writeFakeAGCCollection(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub requires a POSIX shell")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = getcol ]; then\n" +
		"  for a in \"$@\"; do\n" +
		"    case \"$a\" in *FAILCOL*.agc) echo 'getcol: bad archive' 1>&2; exit 1 ;; esac\n" +
		"  done\n" +
		"  printf '>WHOLE1\\nACGTACGT\\n>WHOLE2\\nTTTTGGGG\\n'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "agc"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake agc: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestFetchGenomesWholeArchiveCombine(t *testing.T) {
	writeFakeAGCCollection(t)
	base := t.TempDir()
	archiveDir := filepath.Join(base, "agc")
	seedArchive(t, archiveDir, "batch_a")

	var combined strings.Builder
	// An empty accession slice means "extract the whole batch" (Mode A by-species).
	groups := map[string][]string{"batch_a": nil}
	res, err := FetchGenomes(groups, FetchSpec{Combine: true, Combined: &combined, ArchiveDir: archiveDir, Parallel: 1})
	if err != nil {
		t.Fatalf("FetchGenomes: %v", err)
	}
	if res.Completed != 1 || res.Failed != 0 {
		t.Fatalf("result = %+v, want Completed 1 Failed 0", res)
	}
	out := combined.String()
	if !strings.Contains(out, ">WHOLE1") || !strings.Contains(out, ">WHOLE2") {
		t.Errorf("combined output should hold the whole-archive records, got:\n%s", out)
	}
}

func TestFetchGenomesWholeArchivePerBatchFile(t *testing.T) {
	writeFakeAGCCollection(t)
	base := t.TempDir()
	archiveDir := filepath.Join(base, "agc")
	seedArchive(t, archiveDir, "batch_a")
	outDir := filepath.Join(base, "out")

	groups := map[string][]string{"batch_a": nil}
	res, err := FetchGenomes(groups, FetchSpec{OutputDir: outDir, ArchiveDir: archiveDir, Parallel: 1})
	if err != nil {
		t.Fatalf("FetchGenomes: %v", err)
	}
	if res.Completed != 1 || res.Failed != 0 {
		t.Fatalf("result = %+v, want Completed 1 Failed 0", res)
	}
	// Non-combine whole-archive writes one file per batch: <archive>.fa.
	data, err := os.ReadFile(filepath.Join(outDir, "batch_a.fa"))
	if err != nil {
		t.Fatalf("read batch_a.fa: %v", err)
	}
	if !strings.Contains(string(data), ">WHOLE1") {
		t.Errorf("batch_a.fa missing whole-archive records: %q", data)
	}
}

func TestFetchGenomesWholeArchiveExtractFailure(t *testing.T) {
	writeFakeAGCCollection(t)
	base := t.TempDir()
	archiveDir := filepath.Join(base, "agc")
	seedArchive(t, archiveDir, "FAILCOL_batch")
	outDir := filepath.Join(base, "out")

	groups := map[string][]string{"FAILCOL_batch": nil}
	res, err := FetchGenomes(groups, FetchSpec{OutputDir: outDir, ArchiveDir: archiveDir, Parallel: 1})
	if err != nil {
		t.Fatalf("FetchGenomes: %v", err)
	}
	if res.Completed != 0 || res.Failed != 1 {
		t.Fatalf("result = %+v, want Completed 0 Failed 1", res)
	}
	// Failure is attributed to the batch, and no partial file is left behind.
	if len(res.Errors) != 1 || res.Errors[0].Accession != "FAILCOL_batch" {
		t.Errorf("expected one failure keyed by the batch name, got %v", res.Errors)
	}
	if _, err := os.Stat(filepath.Join(outDir, "FAILCOL_batch.fa")); !os.IsNotExist(err) {
		t.Errorf("partial batch file should be removed on failure")
	}
}

func TestFetchGenomesWholeArchiveDownloadFailure(t *testing.T) {
	writeFakeAGCCollection(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r) // the batch download 404s
	}))
	defer srv.Close()

	base := t.TempDir()
	archiveDir := filepath.Join(base, "agc") // empty: nothing cached, so a download is attempted
	outDir := filepath.Join(base, "out")

	// A download failure for a whole-archive group (empty accs) must still be
	// counted and attributed to the batch, not silently dropped.
	groups := map[string][]string{"missing_batch": nil}
	res, err := FetchGenomes(groups, FetchSpec{OutputDir: outDir, ArchiveDir: archiveDir, BaseURL: srv.URL + "/", Parallel: 1})
	if err != nil {
		t.Fatalf("FetchGenomes: %v", err)
	}
	if res.Completed != 0 || res.Failed != 1 {
		t.Fatalf("result = %+v, want Completed 0 Failed 1", res)
	}
	if len(res.Errors) != 1 || res.Errors[0].Accession != "missing_batch" {
		t.Errorf("download failure should be attributed to the batch name, got %v", res.Errors)
	}
	if !strings.HasPrefix(res.Errors[0].Error, "download missing_batch.agc:") {
		t.Errorf("error = %q, want a 'download missing_batch.agc:' prefix", res.Errors[0].Error)
	}
}

// buildRealArchive writes a tiny single-sample FASTA, builds a real .agc with
// the installed agc binary, and returns (archivePath, sampleName). It skips the
// test when agc is not installed.
func buildRealArchive(t *testing.T) (string, string) {
	t.Helper()
	bin, err := FindBinary()
	if err != nil {
		t.Skip("agc not installed")
	}
	dir := t.TempDir()

	var seq strings.Builder
	const bases = "ACGT"
	for i := 0; i < 1000; i++ {
		seq.WriteByte(bases[i%4])
	}
	faPath := filepath.Join(dir, "sample1.fa")
	if err := os.WriteFile(faPath, []byte(">ctg1\n"+seq.String()+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "src.agc")
	if out, err := exec.Command(bin, "create", "-o", archive, faPath).CombinedOutput(); err != nil {
		t.Fatalf("agc create: %v\n%s", err, out)
	}
	samples, err := ListSamples(archive)
	if err != nil || len(samples) == 0 {
		t.Fatalf("ListSamples: %v (%v)", err, samples)
	}
	return archive, samples[0]
}

func TestFetchGenomesIntegrationRoundTrip(t *testing.T) {
	srcArchive, sample := buildRealArchive(t)
	srcData, err := os.ReadFile(srcArchive)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/genomes_batch.agc" {
			w.Write(srcData)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	base := t.TempDir()
	outDir := filepath.Join(base, "out")
	groups := map[string][]string{"genomes_batch": {sample}}
	res, err := FetchGenomes(groups, FetchSpec{
		OutputDir:  outDir,
		ArchiveDir: filepath.Join(base, "agc"),
		BaseURL:    srv.URL + "/",
		Parallel:   1,
		Options:    Options{Threads: 1},
	})
	if err != nil {
		t.Fatalf("FetchGenomes: %v", err)
	}
	if res.Completed != 1 || res.Failed != 0 {
		t.Fatalf("result = %+v, want Completed 1 Failed 0", res)
	}
	data, err := os.ReadFile(filepath.Join(outDir, sample+".fa"))
	if err != nil {
		t.Fatalf("read extracted FASTA: %v", err)
	}
	if !strings.Contains(string(data), ">") {
		t.Errorf("extracted file is not FASTA: %q", data[:min(40, len(data))])
	}
}

func TestFetchGenomesCombinePartialFailureRealBinary(t *testing.T) {
	srcArchive, sample := buildRealArchive(t)
	srcData, err := os.ReadFile(srcArchive)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(srcData)
	}))
	defer srv.Close()

	var combined strings.Builder
	// In combine mode each sample is extracted independently, so an invalid
	// sample (BOGUS_SAMPLE_NAME) fails only itself while the valid sample is
	// still written to the combined stream — exactly once, with no partial
	// bytes from the failed one.
	groups := map[string][]string{"genomes_batch": {sample, "BOGUS_SAMPLE_NAME"}}
	res, err := FetchGenomes(groups, FetchSpec{
		Combine:    true,
		Combined:   &combined,
		ArchiveDir: filepath.Join(t.TempDir(), "agc"),
		BaseURL:    srv.URL + "/",
		Parallel:   1,
		Options:    Options{Threads: 1},
	})
	if err != nil {
		t.Fatalf("FetchGenomes: %v", err)
	}
	if res.Completed != 1 || res.Failed != 1 {
		t.Fatalf("result = %+v, want Completed 1 Failed 1", res)
	}
	if !strings.Contains(combined.String(), ">") {
		t.Errorf("combined output should contain the valid sample's FASTA")
	}
}
