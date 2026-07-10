package agc

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/allthebacteria/atb-cli/internal/sources"
)

func TestParseMap(t *testing.T) {
	in := "" +
		"SAMD00000344\tatb_batch_1\n" +
		"\n" +
		"SAMD00000345   atb_batch_1   extra_ignored_column\n" +
		"SAMD00000346 atb_batch_2\n" +
		"junk_single_column\n"
	got, err := ParseMap(strings.NewReader(in))
	if err != nil {
		t.Fatalf("ParseMap: %v", err)
	}
	want := map[string]string{
		"SAMD00000344": "atb_batch_1",
		"SAMD00000345": "atb_batch_1",
		"SAMD00000346": "atb_batch_2",
	}
	if len(got) != len(want) {
		t.Fatalf("ParseMap size = %d, want %d (%v)", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("ParseMap[%q] = %q, want %q", k, got[k], v)
		}
	}
}

func TestParseMapExactNotSubstring(t *testing.T) {
	// Unlike the prototype's `grep -F`, lookup is by exact accession, so a
	// prefix must NOT match a longer key.
	m, err := ParseMap(strings.NewReader("SAMD00000344 batch_a\nSAMD00000344X batch_b\n"))
	if err != nil {
		t.Fatal(err)
	}
	if m["SAMD00000344"] != "batch_a" {
		t.Errorf("exact lookup failed: %v", m)
	}
	if _, ok := m["SAMD0000034"]; ok {
		t.Error("partial key must not resolve")
	}
}

func TestFetchMapDownloadsAndCaches(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		io.WriteString(w, "SAMD00000344 atb_batch_1\nSAMD00000345 atb_batch_2\n")
	}))
	defer srv.Close()

	dir := t.TempDir()

	m, err := FetchMap(dir, srv.URL, false)
	if err != nil {
		t.Fatalf("FetchMap: %v", err)
	}
	if m["SAMD00000344"] != "atb_batch_1" {
		t.Errorf("map missing expected entry: %v", m)
	}
	if _, err := os.Stat(filepath.Join(dir, sources.AGCArchiveMapFilename)); err != nil {
		t.Errorf("cache file not written: %v", err)
	}

	// Second call within TTL is a cache hit: no extra HTTP request.
	if _, err := FetchMap(dir, srv.URL, false); err != nil {
		t.Fatalf("FetchMap (cached): %v", err)
	}
	if hits != 1 {
		t.Errorf("expected 1 HTTP hit (cache hit on second call), got %d", hits)
	}

	// force=true re-downloads.
	if _, err := FetchMap(dir, srv.URL, true); err != nil {
		t.Fatalf("FetchMap (force): %v", err)
	}
	if hits != 2 {
		t.Errorf("expected force to re-download (2 hits), got %d", hits)
	}
}

func TestFetchMapStaleCacheRefetches(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		io.WriteString(w, "SAMD1 batch_x\n")
	}))
	defer srv.Close()

	dir := t.TempDir()
	if _, err := FetchMap(dir, srv.URL, false); err != nil {
		t.Fatal(err)
	}
	// Backdate the cache file beyond the TTL.
	old := time.Now().Add(-CacheMaxAge - time.Hour)
	if err := os.Chtimes(filepath.Join(dir, sources.AGCArchiveMapFilename), old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := FetchMap(dir, srv.URL, false); err != nil {
		t.Fatal(err)
	}
	if hits != 2 {
		t.Errorf("expected stale cache to refetch (2 hits), got %d", hits)
	}
}

func TestFetchMapHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()
	if _, err := FetchMap(t.TempDir(), srv.URL, false); err == nil {
		t.Fatal("expected error on HTTP 404, got nil")
	}
}

func seedMap(t *testing.T, dataDir, body string) {
	t.Helper()
	cacheDir := filepath.Join(dataDir, sources.AGCArchiveSubdir)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, sources.AGCArchiveMapFilename), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveArchivesGroupsAndReportsUnresolved(t *testing.T) {
	dataDir := t.TempDir()
	seedMap(t, dataDir,
		"ACC1 batch_a\nACC2 batch_b\nACC3 batch_a\n")

	groups, unresolved, err := ResolveArchives(dataDir, "", []string{"ACC1", "ACC3", "ACC2", "MISSING"}, false)
	if err != nil {
		t.Fatalf("ResolveArchives: %v", err)
	}
	// batch_a holds ACC1 and ACC3, in input order; batch_b holds ACC2.
	if len(groups["batch_a"]) != 2 || groups["batch_a"][0] != "ACC1" || groups["batch_a"][1] != "ACC3" {
		t.Errorf("batch_a group wrong: %v", groups["batch_a"])
	}
	if len(groups["batch_b"]) != 1 || groups["batch_b"][0] != "ACC2" {
		t.Errorf("batch_b group wrong: %v", groups["batch_b"])
	}
	if len(unresolved) != 1 || unresolved[0].Accession != "MISSING" {
		t.Errorf("unresolved wrong: %v", unresolved)
	}
}

func TestResolveArchivesMapUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()
	// force=true skips the (absent) cache and fails on the bad URL.
	if _, _, err := ResolveArchives(t.TempDir(), srv.URL, []string{"ACC1"}, true); err == nil {
		t.Fatal("expected error when map is unreachable, got nil")
	}
}

func zipBytes(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(f, content); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestFetchMapZip(t *testing.T) {
	z := zipBytes(t, "atb202505_files_list.txt", "SAMD1 batch_a\nSAMD2 batch_b\n")
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write(z)
	}))
	defer srv.Close()

	dir := t.TempDir()
	m, err := FetchMap(dir, srv.URL, false)
	if err != nil {
		t.Fatalf("FetchMap(zip): %v", err)
	}
	if m["SAMD1"] != "batch_a" || m["SAMD2"] != "batch_b" {
		t.Errorf("zip map not decompressed/parsed: %v", m)
	}
	// The cache holds the compressed artifact as downloaded, not the expansion.
	cached, err := os.ReadFile(filepath.Join(dir, sources.AGCArchiveMapFilename))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(cached, z) {
		t.Errorf("cache should store the downloaded zip verbatim (%d bytes), got %d", len(z), len(cached))
	}
	// Warm cache still decompresses correctly with no extra download.
	if _, err := FetchMap(dir, srv.URL, false); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Errorf("warm cache should not re-download, hits=%d", hits)
	}
}

func TestFetchMapPlainTextFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "SAMD1 batch_a\n")
	}))
	defer srv.Close()
	m, err := FetchMap(t.TempDir(), srv.URL, false)
	if err != nil {
		t.Fatalf("plain-text FetchMap: %v", err)
	}
	if m["SAMD1"] != "batch_a" {
		t.Errorf("plain-text map not parsed: %v", m)
	}
}

func TestOpenMapEmptyZipErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.zip")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if err := zw.Close(); err != nil { // valid but empty zip
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := openMap(path); err == nil {
		t.Fatal("empty zip must error")
	}
}
