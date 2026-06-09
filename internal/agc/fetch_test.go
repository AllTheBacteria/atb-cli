package agc

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
