package osf

import (
	"bytes"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/allthebacteria/atb-cli/internal/sources"
)

func approxEq(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestParseAGCNodePage(t *testing.T) {
	f, err := os.Open("testdata/agc_node_page1.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	entries, next, err := parseAGCNodePage(f)
	if err != nil {
		t.Fatalf("parseAGCNodePage: %v", err)
	}

	// The folder item must be skipped; only the two .agc files remain.
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2 (folder must be skipped)", len(entries))
	}

	e := entries[0]
	if e.Filename != "Acinetobacter_baylyi_global_ordered_0001.agc" {
		t.Errorf("Filename = %q", e.Filename)
	}
	if e.Project != "Acinetobacter_baylyi" {
		t.Errorf("Project (species) = %q, want Acinetobacter_baylyi", e.Project)
	}
	if e.URL != "https://osf.io/download/6a35c9b1cccfb0aaef166825/" {
		t.Errorf("URL = %q", e.URL)
	}
	if e.MD5 != "7be632ec46828a45a4d6d01d77b8099d" {
		t.Errorf("MD5 = %q", e.MD5)
	}
	if !approxEq(e.SizeMB, float64(3890981)/1e6) {
		t.Errorf("SizeMB = %v, want ~3.890981", e.SizeMB)
	}

	// GTDB letter-suffix species must survive the split.
	if entries[1].Project != "Streptococcus_suis_AA" {
		t.Errorf("entries[1].Project = %q, want Streptococcus_suis_AA", entries[1].Project)
	}

	wantNext := "https://api.osf.io/v2/nodes/z7q5y/files/osfstorage/6a35c88e71b808fa8816675d/?page=2"
	if next != wantNext {
		t.Errorf("next = %q, want %q", next, wantNext)
	}
}

func TestSpeciesFromArchive(t *testing.T) {
	cases := map[string]string{
		"Acinetobacter_baylyi_global_ordered_0001.agc":     "Acinetobacter_baylyi",
		"Streptococcus_suis_AA_global_ordered_0001.agc":    "Streptococcus_suis_AA",
		"Pseudomonas_E_asiatica_global_ordered_0001.agc":   "Pseudomonas_E_asiatica",
		"subthreshold_remainder_global_ordered_0091.agc":   "subthreshold_remainder",
		"Acinetobacter_baylyi_global_ordered_0001":         "Acinetobacter_baylyi", // no extension
		"weird_name_without_token.agc":                     "weird_name_without_token",
	}
	for in, want := range cases {
		if got := SpeciesFromArchive(in); got != want {
			t.Errorf("SpeciesFromArchive(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCrawlAGCNode(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body []byte
		if r.URL.Path == "/p2" {
			body, _ = os.ReadFile("testdata/agc_node_page2.json")
		} else {
			raw, _ := os.ReadFile("testdata/agc_node_page1.json")
			// Point page 1's "next" at this test server's page 2.
			body = []byte(strings.Replace(string(raw),
				"https://api.osf.io/v2/nodes/z7q5y/files/osfstorage/6a35c88e71b808fa8816675d/?page=2",
				server.URL+"/p2", 1))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer server.Close()

	idx, err := CrawlAGCNode(server.Client(), server.URL, "z7q5y")
	if err != nil {
		t.Fatalf("CrawlAGCNode: %v", err)
	}
	if len(idx.Entries) != 3 {
		t.Fatalf("got %d entries across 2 pages, want 3", len(idx.Entries))
	}
	for _, e := range idx.Entries {
		if e.ProjectID != "z7q5y" {
			t.Errorf("ProjectID = %q, want node id z7q5y", e.ProjectID)
		}
	}
	last := idx.Entries[len(idx.Entries)-1]
	if last.Filename != "subthreshold_remainder_global_ordered_0091.agc" {
		t.Errorf("last entry = %q, want the page-2 subthreshold batch", last.Filename)
	}
}

func TestCrawlAGCIndex(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/root":
			raw, _ := os.ReadFile("testdata/agc_node_root.json")
			w.Write([]byte(strings.Replace(string(raw),
				"FOLDER_URL_PLACEHOLDER", server.URL+"/folder", 1)))
		case "/folder":
			raw, _ := os.ReadFile("testdata/agc_node_page1.json")
			w.Write([]byte(strings.Replace(string(raw),
				"https://api.osf.io/v2/nodes/z7q5y/files/osfstorage/6a35c88e71b808fa8816675d/?page=2",
				server.URL+"/p2", 1)))
		case "/p2":
			raw, _ := os.ReadFile("testdata/agc_node_page2.json")
			w.Write(raw)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Resolves the agc_batches folder from the node root, then crawls all pages,
	// with no caching side effect.
	idx, err := CrawlAGCIndex(server.URL+"/root", "z7q5y")
	if err != nil {
		t.Fatalf("CrawlAGCIndex: %v", err)
	}
	if len(idx.Entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(idx.Entries))
	}
	for _, e := range idx.Entries {
		if e.ProjectID != "z7q5y" {
			t.Errorf("ProjectID = %q, want z7q5y", e.ProjectID)
		}
	}
}

func TestFetchAGCIndexCacheFirst(t *testing.T) {
	var hits int
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/root":
			raw, _ := os.ReadFile("testdata/agc_node_root.json")
			// Point the agc_batches folder's related link at this server.
			w.Write([]byte(strings.Replace(string(raw),
				"FOLDER_URL_PLACEHOLDER", server.URL+"/folder", 1)))
		case "/folder":
			raw, _ := os.ReadFile("testdata/agc_node_page1.json")
			w.Write([]byte(strings.Replace(string(raw),
				"https://api.osf.io/v2/nodes/z7q5y/files/osfstorage/6a35c88e71b808fa8816675d/?page=2",
				server.URL+"/p2", 1)))
		case "/p2":
			raw, _ := os.ReadFile("testdata/agc_node_page2.json")
			w.Write(raw)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := t.TempDir()

	// Cold cache: must crawl (folder resolution + 2 pages) and write the TSV.
	idx, err := FetchAGCIndex(dir, server.URL+"/root", "z7q5y", false)
	if err != nil {
		t.Fatalf("FetchAGCIndex (cold): %v", err)
	}
	if len(idx.Entries) != 3 {
		t.Fatalf("cold cache: got %d entries, want 3", len(idx.Entries))
	}
	if hits == 0 {
		t.Fatal("cold cache: expected server to be contacted")
	}
	for _, e := range idx.Entries {
		if e.ProjectID != "z7q5y" {
			t.Errorf("ProjectID = %q, want z7q5y", e.ProjectID)
		}
	}

	cacheFile := filepath.Join(dir, sources.AGCIndexFilename)
	if _, err := os.Stat(cacheFile); err != nil {
		t.Fatalf("cold cache: index TSV not written to %s: %v", cacheFile, err)
	}

	// Warm cache: fresh file on disk, so no further server contact.
	hitsAfterCold := hits
	idx2, err := FetchAGCIndex(dir, server.URL+"/root", "z7q5y", false)
	if err != nil {
		t.Fatalf("FetchAGCIndex (warm): %v", err)
	}
	if len(idx2.Entries) != 3 {
		t.Fatalf("warm cache: got %d entries, want 3", len(idx2.Entries))
	}
	if hits != hitsAfterCold {
		t.Errorf("warm cache made %d extra server hit(s), want 0", hits-hitsAfterCold)
	}

	// force=true must bypass the warm cache and crawl again.
	idx3, err := FetchAGCIndex(dir, server.URL+"/root", "z7q5y", true)
	if err != nil {
		t.Fatalf("FetchAGCIndex (force): %v", err)
	}
	if len(idx3.Entries) != 3 {
		t.Fatalf("force refresh: got %d entries, want 3", len(idx3.Entries))
	}
	if hits == hitsAfterCold {
		t.Error("force refresh did not re-contact the server")
	}
}

func TestFetchAGCIndexFromURL(t *testing.T) {
	const tsv = "project\tproject_id\tfilename\turl\tmd5\tsize_mb\n" +
		"Acinetobacter_baylyi\tz7q5y\tAcinetobacter_baylyi_global_ordered_0001.agc\thttps://osf.io/download/aaa/\t7be632ec46828a45a4d6d01d77b8099d\t3.890981\n" +
		"Salmonella_enterica\tz7q5y\tSalmonella_enterica_global_ordered_0072.agc\thttps://osf.io/download/bbb/\t1650ac20b0da23db315b0c31dc04b8a1\t34.606133\n"

	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path != "/idx" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/tab-separated-values")
		w.Write([]byte(tsv))
	}))
	defer server.Close()

	dir := t.TempDir()
	url := server.URL + "/idx"

	// Cold cache: download the hosted TSV, parse it, and write the cache file.
	idx, err := FetchAGCIndexFromURL(dir, url, false)
	if err != nil {
		t.Fatalf("FetchAGCIndexFromURL (cold): %v", err)
	}
	if len(idx.Entries) != 2 {
		t.Fatalf("cold cache: got %d entries, want 2", len(idx.Entries))
	}
	if idx.Entries[0].URL != "https://osf.io/download/aaa/" {
		t.Errorf("entry URL = %q, want the per-archive OSF download URL", idx.Entries[0].URL)
	}
	if hits != 1 {
		t.Fatalf("cold cache: %d server hits, want 1", hits)
	}
	cacheFile := filepath.Join(dir, sources.AGCIndexFilename)
	if _, err := os.Stat(cacheFile); err != nil {
		t.Fatalf("cold cache: index TSV not written to %s: %v", cacheFile, err)
	}

	// Warm cache: fresh file on disk, so no further download.
	idx2, err := FetchAGCIndexFromURL(dir, url, false)
	if err != nil {
		t.Fatalf("FetchAGCIndexFromURL (warm): %v", err)
	}
	if len(idx2.Entries) != 2 {
		t.Fatalf("warm cache: got %d entries, want 2", len(idx2.Entries))
	}
	if hits != 1 {
		t.Errorf("warm cache made %d extra download(s), want 0", hits-1)
	}

	// force=true must bypass the warm cache and re-download.
	if _, err := FetchAGCIndexFromURL(dir, url, true); err != nil {
		t.Fatalf("FetchAGCIndexFromURL (force): %v", err)
	}
	if hits != 2 {
		t.Errorf("force refresh: %d total hits, want 2", hits)
	}
}

func TestWriteAGCIndexTSVRoundTrip(t *testing.T) {
	idx := &Index{Entries: []Entry{
		{Project: "Acinetobacter_baylyi", ProjectID: "z7q5y",
			Filename: "Acinetobacter_baylyi_global_ordered_0001.agc",
			URL:      "https://osf.io/download/abc/", MD5: "7be632ec46828a45a4d6d01d77b8099d", SizeMB: 3.890981},
		{Project: "subthreshold_remainder", ProjectID: "z7q5y",
			Filename: "subthreshold_remainder_global_ordered_0091.agc",
			URL:      "https://osf.io/download/def/", MD5: "eee2fcce08c28ce4ef6f0199ea69a04f", SizeMB: 114.899074},
	}}

	var buf bytes.Buffer
	if err := WriteAGCIndexTSV(idx, &buf); err != nil {
		t.Fatalf("WriteAGCIndexTSV: %v", err)
	}

	got, err := ParseIndex(&buf)
	if err != nil {
		t.Fatalf("ParseIndex round-trip: %v", err)
	}
	if len(got.Entries) != len(idx.Entries) {
		t.Fatalf("round-trip entry count = %d, want %d", len(got.Entries), len(idx.Entries))
	}
	for i, want := range idx.Entries {
		g := got.Entries[i]
		if g.Project != want.Project || g.ProjectID != want.ProjectID ||
			g.Filename != want.Filename || g.URL != want.URL || g.MD5 != want.MD5 ||
			!approxEq(g.SizeMB, want.SizeMB) {
			t.Errorf("round-trip entry %d = %+v, want %+v", i, g, want)
		}
	}
}
