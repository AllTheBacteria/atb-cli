package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/allthebacteria/atb-cli/internal/osf"
	"github.com/allthebacteria/atb-cli/internal/sources"
)

// writeBatchIndex writes a local AGC batch index TSV covering the named batches
// (each on a collection node) and returns its path, so accession-mode tests can
// resolve batch -> index URL through the bridge without any network I/O.
func writeBatchIndex(t *testing.T, batches ...string) string {
	t.Helper()
	tsv := "project\tproject_id\tfilename\turl\tmd5\tsize_mb\n"
	for _, name := range batches {
		tsv += name + "\tjmeqg\t" + name + ".agc\thttps://osf.io/download/" + name + "/\tmd5" + name + "\t1.000000\n"
	}
	path := filepath.Join(t.TempDir(), "idx.tsv")
	if err := os.WriteFile(path, []byte(tsv), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// seedArchiveMap writes a cached archive map under <dataDir>/agc so the command
// resolves accessions with no network I/O.
func seedArchiveMap(t *testing.T, dataDir, body string) {
	t.Helper()
	cacheDir := filepath.Join(dataDir, sources.AGCArchiveSubdir)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, sources.AGCArchiveMapFilename), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAGCDownloadHelpListsFlags(t *testing.T) {
	stdout, _, err := runCmd("agc", "download", "--help")
	if err != nil {
		t.Fatalf("agc download --help: %v", err)
	}
	for _, want := range []string{"--from", "--combine", "--archive-dir", "--dry-run", "--species", "--agc-index"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected %q in --help, got:\n%s", want, stdout)
		}
	}
}

// TestAGCDownloadHelpAgcIndexDefault guards that the --agc-index help text
// reflects the current default: both modes share the batch index, which by
// default crawls the full OSF collection. The help must mention the collection
// and must not claim the old crawl-only single-node default.
func TestAGCDownloadHelpAgcIndexDefault(t *testing.T) {
	stdout, _, err := runCmd("agc", "download", "--help")
	if err != nil {
		t.Fatalf("agc download --help: %v", err)
	}
	if !strings.Contains(strings.ToLower(stdout), "collection") {
		t.Errorf("expected --agc-index help to mention the collection-crawl default, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "crawl the OSF node and cache it") {
		t.Errorf("stale --agc-index help still claims a crawl-only default:\n%s", stdout)
	}
}

// TestTopLevelFetchGenomesRemoved guards the rename: the command moved under the
// agc group, so the old top-level path must no longer resolve.
func TestTopLevelFetchGenomesRemoved(t *testing.T) {
	_, _, err := runCmd("fetch-genomes", "--help")
	if err == nil {
		t.Fatal("expected top-level 'fetch-genomes' to be gone after the move to 'agc download'")
	}
}

// agcIndexTSV is the 6-column separate AGC index (atb_agc_files.tsv layout): it
// reuses the master-index columns so osf.ParseIndex round-trips it. Tests feed it
// via --agc-index to exercise Mode A without any network I/O.
const agcIndexTSV = "project\tproject_id\tfilename\turl\tmd5\tsize_mb\n" +
	"Acinetobacter_baylyi\t4jq8u\tAcinetobacter_baylyi_global_ordered_0001.agc\thttps://osf.io/download/aaa/\tmd5aaa\t3.890000\n" +
	"Acinetobacter_baylyi\t4jq8u\tAcinetobacter_baylyi_global_ordered_0002.agc\thttps://osf.io/download/bbb/\tmd5bbb\t4.100000\n" +
	"Streptococcus_suis_AA\t4jq8u\tStreptococcus_suis_AA_global_ordered_0001.agc\thttps://osf.io/download/ccc/\tmd5ccc\t10.400000\n"

func writeAGCIndex(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "atb_agc_files.tsv")
	if err := os.WriteFile(path, []byte(agcIndexTSV), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAGCDownloadSpeciesDryRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub requires a POSIX shell")
	}
	dir := t.TempDir()
	withFakeAGC(t, "#!/bin/sh\nexit 0\n")
	idxPath := writeAGCIndex(t)

	stdout, stderr, err := runCmd("agc", "download", "--data-dir", dir,
		"--species", "Acinetobacter baylyi", "--agc-index", idxPath, "--dry-run")
	if err != nil {
		t.Fatalf("species dry-run failed: %v\nstderr: %s", err, stderr)
	}
	combined := stdout + stderr
	// Both A. baylyi batches must be listed; the unrelated species must not.
	for _, want := range []string{
		"Acinetobacter_baylyi_global_ordered_0001",
		"Acinetobacter_baylyi_global_ordered_0002",
	} {
		if !strings.Contains(combined, want) {
			t.Errorf("dry-run should list %q, got:\n%s", want, combined)
		}
	}
	if strings.Contains(combined, "Streptococcus") {
		t.Errorf("dry-run must not list batches of other species, got:\n%s", combined)
	}
	if !strings.Contains(strings.ToLower(combined), "dry-run") {
		t.Errorf("dry-run should announce itself, got:\n%s", combined)
	}
	// Nothing may be downloaded in a dry-run.
	if _, err := os.Stat(filepath.Join(dir, sources.AGCArchiveSubdir, "Acinetobacter_baylyi_global_ordered_0001.agc")); !os.IsNotExist(err) {
		t.Errorf("dry-run must not download archives")
	}
	// stdout is reserved for FASTA payload; diagnostics belong on stderr.
	if stdout != "" {
		t.Errorf("dry-run must keep stdout empty, got:\n%q", stdout)
	}
}

func TestAGCDownloadSpeciesNoMatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub requires a POSIX shell")
	}
	dir := t.TempDir()
	withFakeAGC(t, "#!/bin/sh\nexit 0\n")
	idxPath := writeAGCIndex(t)

	_, stderr, err := runCmd("agc", "download", "--data-dir", dir,
		"--species", "Escherichia coli", "--agc-index", idxPath, "--dry-run")
	if err == nil {
		t.Fatal("expected an error when no batch matches the species")
	}
	msg := err.Error() + stderr
	if !strings.Contains(msg, "Escherichia coli") {
		t.Errorf("error should name the species, got: %v / %s", err, stderr)
	}
}

func TestAGCDownloadSpeciesRejectsAccessions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub requires a POSIX shell")
	}
	dir := t.TempDir()
	withFakeAGC(t, "#!/bin/sh\nexit 0\n")
	idxPath := writeAGCIndex(t)

	// --species is its own access mode; mixing it with accessions is ambiguous.
	_, _, err := runCmd("agc", "download", "--data-dir", dir,
		"--species", "Acinetobacter baylyi", "--agc-index", idxPath, "SAMD00000344")
	if err == nil {
		t.Fatal("expected an error when --species is combined with accession arguments")
	}
	if !strings.Contains(err.Error(), "--species") {
		t.Errorf("error should mention --species, got: %v", err)
	}
}

func TestAGCDownloadDryRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub requires a POSIX shell")
	}
	dir := t.TempDir()
	// Install a fake agc so FindBinary passes.
	withFakeAGC(t, "#!/bin/sh\nexit 0\n")
	seedArchiveMap(t, dir, "ACC1 batch_a\nACC2 batch_a\n")
	idxPath := writeBatchIndex(t, "batch_a")

	stdout, stderr, err := runCmd("agc", "download", "--data-dir", dir, "--agc-index", idxPath, "--dry-run", "ACC1", "ACC2")
	if err != nil {
		t.Fatalf("dry-run failed: %v\nstderr: %s", err, stderr)
	}
	combined := stdout + stderr
	if !strings.Contains(combined, "batch_a") {
		t.Errorf("dry-run should name the archive, got:\n%s", combined)
	}
	if !strings.Contains(strings.ToLower(combined), "dry-run") {
		t.Errorf("dry-run should announce itself, got:\n%s", combined)
	}
	// No archive should have been downloaded.
	if _, err := os.Stat(filepath.Join(dir, sources.AGCArchiveSubdir, "batch_a.agc")); !os.IsNotExist(err) {
		t.Errorf("dry-run must not download archives")
	}
}

func TestAGCDownloadUnresolvedWarns(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub requires a POSIX shell")
	}
	dir := t.TempDir()
	withFakeAGC(t, "#!/bin/sh\nexit 0\n")
	seedArchiveMap(t, dir, "ACC1 batch_a\n")
	idxPath := writeBatchIndex(t, "batch_a")

	_, stderr, err := runCmd("agc", "download", "--data-dir", dir, "--agc-index", idxPath, "--dry-run", "ACC1", "GHOST")
	if err == nil {
		t.Fatal("expected non-zero exit when an accession is unresolved")
	}
	if !strings.Contains(stderr, "GHOST") {
		t.Errorf("expected a warning naming GHOST, got:\n%s", stderr)
	}
}

func TestAGCDownloadMissingBinary(t *testing.T) {
	dir := t.TempDir()
	seedArchiveMap(t, dir, "ACC1 batch_a\n")
	// Empty PATH so FindBinary cannot locate agc, and the test binary's own
	// directory has no agc either.
	t.Setenv("PATH", t.TempDir())

	_, _, err := runCmd("agc", "download", "--data-dir", dir, "ACC1")
	if err == nil {
		t.Fatal("expected error when agc is not installed")
	}
	if !strings.Contains(err.Error(), "agc install") {
		t.Errorf("error should point at 'atb agc install', got: %v", err)
	}
}

func TestAGCDownloadDryRunMultipleArchives(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub requires a POSIX shell")
	}
	dir := t.TempDir()
	withFakeAGC(t, "#!/bin/sh\nexit 0\n")
	seedArchiveMap(t, dir, "ACC1 batch_a\nACC2 batch_b\nACC3 batch_a\n")
	idxPath := writeBatchIndex(t, "batch_a", "batch_b")

	stdout, stderr, err := runCmd("agc", "download", "--data-dir", dir, "--agc-index", idxPath, "--dry-run", "ACC1", "ACC2", "ACC3")
	if err != nil {
		t.Fatalf("dry-run failed: %v\nstderr: %s", err, stderr)
	}
	// Both archives must be listed.
	for _, want := range []string{"batch_a", "batch_b"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("dry-run should list %q, got:\n%s", want, stderr)
		}
	}
	// 3 samples spread across 2 archives.
	if !strings.Contains(stderr, "2 archive(s) for 3 sample(s)") {
		t.Errorf("dry-run summary wrong, got:\n%s", stderr)
	}
	// Diagnostics must never reach stdout, which is reserved for FASTA payload
	// (e.g. --combine piped to another tool).
	if stdout != "" {
		t.Errorf("dry-run must keep stdout empty, got:\n%q", stdout)
	}
}

func TestAGCDownloadDedupesArgsAndFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub requires a POSIX shell")
	}
	dir := t.TempDir()
	withFakeAGC(t, "#!/bin/sh\nexit 0\n")
	seedArchiveMap(t, dir, "ACC1 batch_a\nACC2 batch_a\n")
	idxPath := writeBatchIndex(t, "batch_a")

	from := filepath.Join(t.TempDir(), "more.txt")
	if err := os.WriteFile(from, []byte("ACC2\nACC3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// args [ACC1 ACC2] + --from [ACC2 ACC3]: ACC2 is duplicated and must
	// collapse to one sample; ACC3 is unresolved and must warn + exit non-zero.
	_, stderr, err := runCmd("agc", "download", "--data-dir", dir, "--agc-index", idxPath, "--dry-run", "ACC1", "ACC2", "--from", from)
	if err == nil {
		t.Fatal("expected non-zero exit for unresolved ACC3")
	}
	// batch_a holds the deduped ACC1+ACC2 = 2 samples, not 3.
	if !strings.Contains(stderr, "1 archive(s) for 2 sample(s)") {
		t.Errorf("expected deduped count 2 in batch_a, got:\n%s", stderr)
	}
	if !strings.Contains(stderr, "ACC3") {
		t.Errorf("expected ACC3 unresolved warning, got:\n%s", stderr)
	}
}

// agcCollectionTSV is a 6-column collection-index TSV whose project_id column
// holds a collection node id; it is shared by the download and locate tests.
const agcCollectionTSV = "project\tproject_id\tfilename\turl\tmd5\tsize_mb\n" +
	"Escherichia_coli\tjmeqg\tEscherichia_coli_global_ordered_0001.agc\thttps://osf.io/download/ec1/\tmd5ec\t1.000000\n" +
	"Salmonella_enterica\tkzcnr\tSalmonella_enterica_global_ordered_0072.agc\thttps://osf.io/download/se72/\tmd5se\t2.000000\n"

// seedAGCIndexCache writes a warm AGC index cache under <dataDir>/agc with the
// given source marker in the ".source" sidecar, so loadAGCBatchIndex serves it
// with no network I/O. The marker must equal what the resolving path expects: the
// hosted URL for the URL path, or CollectionCacheSource(nodes) for the crawl.
func seedAGCIndexCache(t *testing.T, dataDir, tsv, marker string) {
	t.Helper()
	cacheDir := filepath.Join(dataDir, sources.AGCArchiveSubdir)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cached := filepath.Join(cacheDir, sources.AGCIndexFilename)
	if err := os.WriteFile(cached, []byte(tsv), 0o644); err != nil {
		t.Fatal(err)
	}
	// ".source" mirrors osf's internal cache sidecar suffix.
	if err := os.WriteFile(cached+".source", []byte(marker+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// seedAGCDefaultIndex warms the cache for loadAGCBatchIndex's default path,
// stamping the marker that path resolves to: the hosted URL when
// sources.AGCIndexURL is set, else the collection-crawl marker.
func seedAGCDefaultIndex(t *testing.T, dataDir, tsv string) {
	t.Helper()
	marker := sources.AGCIndexURL
	if marker == "" {
		marker = osf.CollectionCacheSource(sources.AGCCollectionNodes)
	}
	seedAGCIndexCache(t, dataDir, tsv, marker)
}

func TestLoadAGCBatchIndexEmptyURLUsesCollectionCache(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, sources.AGCArchiveSubdir)
	// With no hosted index URL, loadAGCBatchIndex falls back to the collection
	// crawl; a cache stamped with the collection marker serves it with zero
	// network I/O (a real crawl would hit api.osf.io).
	seedAGCIndexCache(t, dir, agcCollectionTSV, osf.CollectionCacheSource(sources.AGCCollectionNodes))
	idx, err := loadAGCBatchIndex("", "", cacheDir, false)
	if err != nil {
		t.Fatalf("loadAGCBatchIndex (empty URL): %v", err)
	}
	if len(idx.Entries) != 2 {
		t.Fatalf("got %d entries, want 2 from the seeded collection cache", len(idx.Entries))
	}
}

// TestLoadAGCBatchIndexURLPathDownloads guards the hosted-index wiring: a
// non-empty index URL must route through FetchAGCIndexFromURL (download the single
// published TSV) rather than the collection crawl.
func TestLoadAGCBatchIndexURLPathDownloads(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		io.WriteString(w, agcCollectionTSV)
	}))
	defer srv.Close()

	cacheDir := filepath.Join(t.TempDir(), sources.AGCArchiveSubdir)
	idx, err := loadAGCBatchIndex("", srv.URL, cacheDir, false)
	if err != nil {
		t.Fatalf("loadAGCBatchIndex (URL path): %v", err)
	}
	if hits != 1 {
		t.Fatalf("hosted index: %d downloads, want 1", hits)
	}
	if len(idx.Entries) != 2 {
		t.Fatalf("got %d entries, want 2 from the served index", len(idx.Entries))
	}
}

func TestLoadAGCBatchIndexLocalPathWins(t *testing.T) {
	local := writeAGCIndex(t) // existing helper: 3-row TSV
	idx, err := loadAGCBatchIndex(local, sources.AGCIndexURL, t.TempDir(), false)
	if err != nil {
		t.Fatalf("loadAGCBatchIndex (local): %v", err)
	}
	if len(idx.Entries) != 3 {
		t.Fatalf("local path: got %d, want 3", len(idx.Entries))
	}
}

func TestAGCDownloadAccessionNotYetAvailable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub requires a POSIX shell")
	}
	dir := t.TempDir()
	withFakeAGC(t, "#!/bin/sh\nexit 0\n")
	// The map knows the batch, but the index (via --agc-index) does not list it.
	seedArchiveMap(t, dir, "ACC1 Ghost_global_ordered_9999\n")
	idxPath := writeAGCIndex(t) // A. baylyi / S. suis rows only, no Ghost batch

	_, stderr, err := runCmd("agc", "download", "--data-dir", dir,
		"--agc-index", idxPath, "--dry-run", "ACC1")
	if err == nil {
		t.Fatal("expected non-zero exit when the batch is not yet available")
	}
	if !strings.Contains(strings.ToLower(stderr), "not yet available") {
		t.Errorf("expected a 'not yet available' warning, got:\n%s", stderr)
	}
	if !strings.Contains(stderr, "Ghost_global_ordered_9999") {
		t.Errorf("warning should name the batch, got:\n%s", stderr)
	}
}

func TestAGCDownloadAccessionOverOSF(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub requires a POSIX shell")
	}
	// Serve the .agc archive from a fake OSF; the index ref points at it.
	var served int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served++
		io.WriteString(w, "fake-agc-bytes")
	}))
	defer srv.Close()

	dir := t.TempDir()
	withFakeAGC(t, "#!/bin/sh\nexit 0\n")
	seedArchiveMap(t, dir, "ACC1 Escherichia_coli_global_ordered_0001\n")
	// A local batch index whose single row's URL is the fake server (md5 empty
	// skips verification). This is the bridge: accession -> batch -> index URL.
	idxTSV := "project\tproject_id\tfilename\turl\tmd5\tsize_mb\n" +
		"Escherichia_coli\tjmeqg\tEscherichia_coli_global_ordered_0001.agc\t" + srv.URL + "/arch\t\t1.000000\n"
	idxPath := filepath.Join(t.TempDir(), "idx.tsv")
	if err := os.WriteFile(idxPath, []byte(idxTSV), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")

	_, stderr, err := runCmd("agc", "download", "--data-dir", dir,
		"--agc-index", idxPath, "-o", out, "ACC1")
	if err != nil {
		t.Fatalf("download over OSF failed: %v\nstderr: %s", err, stderr)
	}
	if served == 0 {
		t.Error("the archive was not downloaded from the index URL (bridge not wired)")
	}
	if !strings.Contains(stderr, "Completed: 1") {
		t.Errorf("want Completed: 1, got:\n%s", stderr)
	}
	if _, err := os.Stat(filepath.Join(out, "ACC1.fa")); err != nil {
		t.Errorf("per-sample FASTA not written: %v", err)
	}
}
