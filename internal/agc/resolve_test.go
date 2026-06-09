package agc

import (
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
