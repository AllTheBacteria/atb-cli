package osf

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
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
	if e.Project != "" {
		t.Errorf("Project = %q, want empty (species comes from the metadata join)", e.Project)
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

	// parseAGCNodePage no longer guesses a species; the metadata join fills it.
	if entries[1].Project != "" {
		t.Errorf("entries[1].Project = %q, want empty (species comes from the join)", entries[1].Project)
	}

	wantNext := "https://api.osf.io/v2/nodes/z7q5y/files/osfstorage/6a35c88e71b808fa8816675d/?page=2"
	if next != wantNext {
		t.Errorf("next = %q, want %q", next, wantNext)
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

func TestCrawlAGCNodeRequestsStableOrder(t *testing.T) {
	// The crawl must page through OSF's stable total order (sort=name,
	// page[size]=100) so page-number pagination cannot gap or overlap. The handler
	// serves 25 distinct files across 3 pages and asserts every request - including
	// the ones reached via "next" - carries the stable-order params.
	const total = 25
	const perPage = 10
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("sort"); got != "name" {
			t.Errorf("request %q: sort = %q, want \"name\"", r.URL.RequestURI(), got)
		}
		if got := r.URL.Query().Get("page[size]"); got != "100" {
			t.Errorf("request %q: page[size] = %q, want \"100\"", r.URL.RequestURI(), got)
		}
		page := 1
		if p := r.URL.Query().Get("page"); p != "" {
			page, _ = strconv.Atoi(p)
		}
		start := (page-1)*perPage + 1
		end := start + perPage - 1
		if end > total {
			end = total
		}
		items := make([]string, 0, perPage)
		for n := start; n <= end; n++ {
			name := fmt.Sprintf("atb.assembly.202505_all.batch.%04d.agc", n)
			items = append(items, fmt.Sprintf(
				`{"attributes":{"name":%q,"kind":"file","size":1000000,`+
					`"extra":{"hashes":{"md5":"abc"}}},"links":{"download":"https://osf.io/download/%d/"}}`,
				name, n))
		}
		next := "null"
		if end < total {
			next = `"` + server.URL + "/folder?sort=name&page%5Bsize%5D=100&page=" +
				strconv.Itoa(page+1) + `"`
		}
		fmt.Fprintf(w, `{"data":[%s],"links":{"next":%s}}`, strings.Join(items, ","), next)
	}))
	defer server.Close()

	idx, err := CrawlAGCNode(server.Client(), server.URL+"/folder", "4jq8u")
	if err != nil {
		t.Fatalf("CrawlAGCNode: %v", err)
	}
	if len(idx.Entries) != total {
		t.Fatalf("got %d entries across pages, want %d", len(idx.Entries), total)
	}
	seen := make(map[string]bool, total)
	for _, e := range idx.Entries {
		if seen[e.Filename] {
			t.Errorf("filename %q returned more than once", e.Filename)
		}
		seen[e.Filename] = true
	}
	if len(seen) != total {
		t.Errorf("distinct filenames = %d, want %d", len(seen), total)
	}
}

func TestFindFolderURLFollowsPagination(t *testing.T) {
	// The target folder can appear on any page of the node root listing, so
	// findFolderURL must follow "next" across pages instead of reading only page 1.
	folderItem := func(name, href string) string {
		return `{"attributes":{"name":"` + name + `","kind":"folder"},` +
			`"relationships":{"files":{"links":{"related":{"href":"` + href + `"}}}}}`
	}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/root":
			// Page 1 holds an unrelated folder and points "next" at page 2.
			fmt.Fprintf(w, `{"data":[%s],"links":{"next":%q}}`,
				folderItem("some_other_folder", server.URL+"/other/"), server.URL+"/root2")
		case "/root2":
			// Page 2 holds the target folder.
			fmt.Fprintf(w, `{"data":[%s],"links":{"next":null}}`,
				folderItem(sources.AGCArchivesFolder, server.URL+"/folder/"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	href, err := findFolderURL(server.Client(), server.URL+"/root", sources.AGCArchivesFolder)
	if err != nil {
		t.Fatalf("findFolderURL must follow pagination to a later page, got error: %v", err)
	}
	if want := server.URL + "/folder/"; href != want {
		t.Errorf("href = %q, want %q", href, want)
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

func TestFetchAGCIndexFromURLInvalidatesOnURLChange(t *testing.T) {
	const tsvA = "project\tproject_id\tfilename\turl\tmd5\tsize_mb\n" +
		"Acinetobacter_baylyi\tz7q5y\tAcinetobacter_baylyi_global_ordered_0001.agc\thttps://osf.io/download/aaa/\t7be632ec46828a45a4d6d01d77b8099d\t3.890981\n" +
		"Salmonella_enterica\tz7q5y\tSalmonella_enterica_global_ordered_0072.agc\thttps://osf.io/download/bbb/\t1650ac20b0da23db315b0c31dc04b8a1\t34.606133\n"
	const tsvB = "project\tproject_id\tfilename\turl\tmd5\tsize_mb\n" +
		"Mycoplasmoides_pneumoniae\tz7q5y\tMycoplasmoides_pneumoniae_global_ordered_0001.agc\thttps://osf.io/download/ccc/\tabc123abc123abc123abc123abc12345\t0.981234\n"

	var hitsA, hitsB int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/tab-separated-values")
		switch r.URL.Path {
		case "/idxA":
			hitsA++
			w.Write([]byte(tsvA))
		case "/idxB":
			hitsB++
			w.Write([]byte(tsvB))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	urlA := server.URL + "/idxA"
	urlB := server.URL + "/idxB"

	// Populate the cache from urlA.
	idxA, err := FetchAGCIndexFromURL(dir, urlA, false)
	if err != nil {
		t.Fatalf("FetchAGCIndexFromURL(urlA): %v", err)
	}
	if len(idxA.Entries) != 2 {
		t.Fatalf("urlA: got %d entries, want 2", len(idxA.Entries))
	}
	if hitsA != 1 {
		t.Fatalf("urlA cold: %d hits, want 1", hitsA)
	}

	// The cache file is still fresh, but the URL changed (the release scenario:
	// a new binary points AGCIndexURL at a freshly published TSV). A URL-keyed
	// cache must re-download from urlB rather than serve the stale urlA snapshot.
	idxB, err := FetchAGCIndexFromURL(dir, urlB, false)
	if err != nil {
		t.Fatalf("FetchAGCIndexFromURL(urlB): %v", err)
	}
	if hitsB != 1 {
		t.Fatalf("urlB with fresh but stale-source cache: %d hits, want 1 (cache not invalidated on URL change)", hitsB)
	}
	if len(idxB.Entries) != 1 || idxB.Entries[0].Project != "Mycoplasmoides_pneumoniae" {
		t.Fatalf("urlB: got %+v, want the single Mycoplasmoides_pneumoniae entry", idxB.Entries)
	}

	// urlB is now the cached source: a repeat fetch serves it with no download.
	if _, err := FetchAGCIndexFromURL(dir, urlB, false); err != nil {
		t.Fatalf("FetchAGCIndexFromURL(urlB warm): %v", err)
	}
	if hitsB != 1 {
		t.Errorf("urlB warm: %d hits, want 1 (should reuse the urlB cache)", hitsB)
	}
}

func TestFetchAGCIndexFromURLMissingSidecarRefetches(t *testing.T) {
	const tsv = "project\tproject_id\tfilename\turl\tmd5\tsize_mb\n" +
		"Acinetobacter_baylyi\tz7q5y\tAcinetobacter_baylyi_global_ordered_0001.agc\thttps://osf.io/download/aaa/\t7be632ec46828a45a4d6d01d77b8099d\t3.890981\n"

	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "text/tab-separated-values")
		w.Write([]byte(tsv))
	}))
	defer server.Close()

	dir := t.TempDir()

	// Simulate a cache written by an older atb that predates source markers: a
	// fresh TSV on disk, but no .source sidecar next to it (the upgrade path).
	cacheFile := filepath.Join(dir, sources.AGCIndexFilename)
	if err := os.WriteFile(cacheFile, []byte(tsv), 0644); err != nil {
		t.Fatal(err)
	}

	// The cache is fresh, but with no marker atb cannot prove it matches the
	// requested URL, so it must re-download rather than serve an unknown source.
	if _, err := FetchAGCIndexFromURL(dir, server.URL+"/idx", false); err != nil {
		t.Fatalf("FetchAGCIndexFromURL: %v", err)
	}
	if hits != 1 {
		t.Fatalf("legacy cache without source marker: %d downloads, want 1", hits)
	}

	// The marker now exists, so a second fetch reuses the cache with no download.
	if _, err := FetchAGCIndexFromURL(dir, server.URL+"/idx", false); err != nil {
		t.Fatalf("FetchAGCIndexFromURL (warm): %v", err)
	}
	if hits != 1 {
		t.Errorf("after marker written: %d downloads, want 1", hits)
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

func TestFirstDuplicateBatch(t *testing.T) {
	// A repeated (project_id, filename) is never legitimate: it means the crawl saw
	// the same batch twice. FirstDuplicateBatch reports the first such key.
	dup := &Index{Entries: []Entry{
		{ProjectID: "4jq8u", Filename: "atb.assembly.202505_all.batch.0001.agc"},
		{ProjectID: "jmeqg", Filename: "atb.assembly.202505_all.batch.0002.agc"},
		{ProjectID: "4jq8u", Filename: "atb.assembly.202505_all.batch.0001.agc"},
	}}
	key, ok := FirstDuplicateBatch(dup)
	if !ok {
		t.Errorf("ok = false, want true on a repeated (project_id, filename)")
	}
	if want := "4jq8u/atb.assembly.202505_all.batch.0001.agc"; key != want {
		t.Errorf("key = %q, want %q", key, want)
	}

	// The same filename under a different node is a distinct batch, not a duplicate.
	distinct := &Index{Entries: []Entry{
		{ProjectID: "4jq8u", Filename: "atb.assembly.202505_all.batch.0001.agc"},
		{ProjectID: "jmeqg", Filename: "atb.assembly.202505_all.batch.0001.agc"},
		{ProjectID: "4jq8u", Filename: "atb.assembly.202505_all.batch.0002.agc"},
	}}
	if key, ok := FirstDuplicateBatch(distinct); ok {
		t.Errorf("all-distinct index: got (%q, true), want (\"\", false)", key)
	}
}

// osfFolderRoot renders an OSF node root listing with a single folder whose
// "related" contents link points at folderURL.
func osfFolderRoot(folderName, folderURL string) string {
	return `{"data":[{"attributes":{"name":"` + folderName + `","kind":"folder"},` +
		`"relationships":{"files":{"links":{"related":{"href":"` + folderURL + `"}}}}}],` +
		`"links":{"next":null}}`
}

// osfFilePage renders a one-file OSF folder listing page (no further pages).
func osfFilePage(filename, downloadURL, md5 string, size int) string {
	return fmt.Sprintf(`{"data":[{"attributes":{"name":%q,"kind":"file","size":%d,`+
		`"extra":{"hashes":{"md5":%q}}},"links":{"download":%q}}],"links":{"next":null}}`,
		filename, size, md5, downloadURL)
}

func TestCrawlAGCCollection(t *testing.T) {
	var srvA, srvB *httptest.Server
	srvA = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/root":
			io.WriteString(w, osfFolderRoot(sources.AGCArchivesFolder, srvA.URL+"/folder"))
		case "/folder":
			io.WriteString(w, osfFilePage("Escherichia_coli_global_ordered_0001.agc", "https://osf.io/download/ec1/", "md5ec", 1000000))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srvA.Close()
	srvB = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/root":
			io.WriteString(w, osfFolderRoot(sources.AGCArchivesFolder, srvB.URL+"/folder"))
		case "/folder":
			io.WriteString(w, osfFilePage("Salmonella_enterica_global_ordered_0072.agc", "https://osf.io/download/se72/", "md5se", 2000000))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srvB.Close()

	nodes := []sources.AGCNode{{ID: "nodeA"}, {ID: "nodeB"}}
	rootURLFor := func(id string) string {
		if id == "nodeA" {
			return srvA.URL + "/root"
		}
		return srvB.URL + "/root"
	}
	idx, err := CrawlAGCCollection(rootURLFor, nodes)
	if err != nil {
		t.Fatalf("CrawlAGCCollection: %v", err)
	}
	if len(idx.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(idx.Entries))
	}
	byNode := map[string]Entry{}
	for _, e := range idx.Entries {
		byNode[e.ProjectID] = e
	}
	if byNode["nodeA"].Filename != "Escherichia_coli_global_ordered_0001.agc" {
		t.Errorf("nodeA entry wrong: %+v", byNode["nodeA"])
	}
	if byNode["nodeB"].Filename != "Salmonella_enterica_global_ordered_0072.agc" {
		t.Errorf("nodeB entry wrong: %+v", byNode["nodeB"])
	}
	if byNode["nodeB"].Project != "" {
		t.Errorf("crawl must leave Project empty (the join fills it), got %+v", byNode["nodeB"])
	}
}

func TestCrawlAGCCollectionSkipsMissingFolder(t *testing.T) {
	var good *httptest.Server
	good = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/root":
			io.WriteString(w, osfFolderRoot(sources.AGCArchivesFolder, good.URL+"/folder"))
		case "/folder":
			io.WriteString(w, osfFilePage("Escherichia_coli_global_ordered_0001.agc", "https://osf.io/download/ec1/", "md5ec", 1000000))
		}
	}))
	defer good.Close()
	// This node's root has no agc_archives folder yet (still provisioning).
	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data":[],"links":{"next":null}}`)
	}))
	defer empty.Close()

	nodes := []sources.AGCNode{{ID: "good"}, {ID: "empty"}}
	rootURLFor := func(id string) string {
		if id == "good" {
			return good.URL + "/root"
		}
		return empty.URL + "/root"
	}
	idx, err := CrawlAGCCollection(rootURLFor, nodes)
	if err != nil {
		t.Fatalf("a still-provisioning node must be skipped, got error: %v", err)
	}
	if len(idx.Entries) != 1 || idx.Entries[0].ProjectID != "good" {
		t.Fatalf("want only the good node's single entry, got %+v", idx.Entries)
	}
}

func TestCrawlAGCCollectionSurfacesNetworkError(t *testing.T) {
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer down.Close()
	nodes := []sources.AGCNode{{ID: "down"}}
	_, err := CrawlAGCCollection(func(string) string { return down.URL + "/root" }, nodes)
	if err == nil {
		t.Fatal("an HTTP 500 on a node must surface as an error, not be silently skipped")
	}
}

func TestCollectionCacheSource(t *testing.T) {
	a := CollectionCacheSource([]sources.AGCNode{{ID: "x"}, {ID: "y"}})
	b := CollectionCacheSource([]sources.AGCNode{{ID: "x"}, {ID: "z"}})
	if a == b {
		t.Errorf("different node sets must yield different markers: %q == %q", a, b)
	}
	if !strings.Contains(a, "x") || !strings.Contains(a, "y") {
		t.Errorf("marker should name the nodes, got %q", a)
	}
}

func TestFetchAGCCollectionCacheFirst(t *testing.T) {
	var hits int
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		switch r.URL.Path {
		case "/root":
			io.WriteString(w, osfFolderRoot(sources.AGCArchivesFolder, srv.URL+"/folder"))
		case "/folder":
			io.WriteString(w, osfFilePage("Escherichia_coli_global_ordered_0001.agc", "https://osf.io/download/ec1/", "md5ec", 1000000))
		case "/metadata":
			w.Write(gzipTSV(t, "batch_name\told_name\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	nodes := []sources.AGCNode{{ID: "only"}}
	rootURLFor := func(string) string { return srv.URL + "/root" }
	metadataURL := srv.URL + "/metadata"

	idx, err := FetchAGCCollection(dir, rootURLFor, nodes, metadataURL, false)
	if err != nil {
		t.Fatalf("cold: %v", err)
	}
	if len(idx.Entries) != 1 {
		t.Fatalf("cold: got %d, want 1", len(idx.Entries))
	}
	if _, err := os.Stat(filepath.Join(dir, sources.AGCIndexFilename)); err != nil {
		t.Fatalf("cold: index TSV not written: %v", err)
	}
	coldHits := hits

	if _, err := FetchAGCCollection(dir, rootURLFor, nodes, metadataURL, false); err != nil {
		t.Fatalf("warm: %v", err)
	}
	if hits != coldHits {
		t.Errorf("warm cache made %d extra hit(s), want 0", hits-coldHits)
	}

	if _, err := FetchAGCCollection(dir, rootURLFor, nodes, metadataURL, true); err != nil {
		t.Fatalf("force: %v", err)
	}
	if hits == coldHits {
		t.Error("force must re-crawl")
	}
}

func TestFetchAGCCollectionInvalidatesOnNodeSetChange(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/root":
			io.WriteString(w, osfFolderRoot(sources.AGCArchivesFolder, srv.URL+"/folder"))
		case "/folder":
			io.WriteString(w, osfFilePage("Escherichia_coli_global_ordered_0001.agc", "https://osf.io/download/ec1/", "md5ec", 1000000))
		case "/metadata":
			w.Write(gzipTSV(t, "batch_name\told_name\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	rootURLFor := func(string) string { return srv.URL + "/root" }
	metadataURL := srv.URL + "/metadata"

	if _, err := FetchAGCCollection(dir, rootURLFor, []sources.AGCNode{{ID: "a"}}, metadataURL, false); err != nil {
		t.Fatal(err)
	}
	idx, err := FetchAGCCollection(dir, rootURLFor, []sources.AGCNode{{ID: "a"}, {ID: "b"}}, metadataURL, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Entries) != 2 {
		t.Fatalf("node-set change must re-crawl both nodes, got %d entries", len(idx.Entries))
	}
}

func TestSpeciesFromOldName(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"major-with-agc", "Escherichia_coli_global_ordered.part001.agc", "Escherichia_coli"},
		{"major-no-agc", "Escherichia_coli_global_ordered.part001", "Escherichia_coli"},
		{"gtdb-suffix", "Streptococcus_suis_AA_global_ordered.part007", "Streptococcus_suis_AA"},
		{"unknown", "unknown.part042", "unknown"},
		{"mixed", "mixed_species.part113.agc", "mixed_species"},
	}
	for _, c := range cases {
		if got := SpeciesFromOldName(c.in); got != c.want {
			t.Errorf("%s: SpeciesFromOldName(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestParseBatchMetadata(t *testing.T) {
	// Column order is intentionally not batch_name-first, to prove header-driven
	// column resolution. A row missing old_name is skipped.
	tsv := "old_name\tbatch_name\tnb_genomes\n" +
		"Escherichia_coli_global_ordered.part001\tatb.assembly.202505_all.batch.0001\t25\n" +
		"unknown.part002\tatb.assembly.202505_all.batch.0002\t25\n" +
		"mixed_species.part004\tatb.assembly.202505_all.batch.0004.agc\t25\n" +
		"\tatb.assembly.202505_all.batch.0003\t25\n"
	m, err := ParseBatchMetadata(strings.NewReader(tsv))
	if err != nil {
		t.Fatalf("ParseBatchMetadata: %v", err)
	}
	if got := m["atb.assembly.202505_all.batch.0001"]; got != "Escherichia_coli_global_ordered.part001" {
		t.Errorf("batch.0001 old_name: got %q", got)
	}
	if got := m["atb.assembly.202505_all.batch.0002"]; got != "unknown.part002" {
		t.Errorf("batch.0002 old_name: got %q", got)
	}
	if got := m["atb.assembly.202505_all.batch.0004"]; got != "mixed_species.part004" {
		t.Errorf("batch.0004 (batch_name carried .agc) must key by stem: got %q", got)
	}
	if _, ok := m["atb.assembly.202505_all.batch.0003"]; ok {
		t.Errorf("batch.0003 has empty old_name and must be skipped")
	}
}

// gzipTSV gzips s for a metadata endpoint. Kept local to the osf test package.
func gzipTSV(t *testing.T, s string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(s)); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

// buildJoinServer serves two collection nodes (root listing + agc_batches folder
// contents) plus a gzipped metadata endpoint, all off one httptest server. Node
// A holds batch.0001 (metadata match) and batch.0999 (no metadata → unmatched);
// node B holds batch.0002.
func buildJoinServer(t *testing.T) *httptest.Server {
	t.Helper()
	folder := sources.AGCArchivesFolder
	fileItem := func(name string) string {
		return `{"attributes":{"name":"` + name + `","kind":"file","size":1000000,` +
			`"extra":{"hashes":{"md5":"abc"}}},"links":{"download":"https://osf.io/download/` + name + `/"}}`
	}
	folderItem := func(nodeHost string) string {
		return `{"attributes":{"name":"` + folder + `","kind":"folder"},` +
			`"relationships":{"files":{"links":{"related":{"href":"` + nodeHost + `/folder/"}}}}}`
	}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nodeA/root/":
			fmt.Fprintf(w, `{"data":[%s],"links":{"next":null}}`, folderItem(srv.URL+"/nodeA"))
		case "/nodeA/folder/":
			fmt.Fprintf(w, `{"data":[%s,%s],"links":{"next":null}}`,
				fileItem("atb.assembly.202505_all.batch.0001.agc"),
				fileItem("atb.assembly.202505_all.batch.0999.agc"))
		case "/nodeB/root/":
			fmt.Fprintf(w, `{"data":[%s],"links":{"next":null}}`, folderItem(srv.URL+"/nodeB"))
		case "/nodeB/folder/":
			fmt.Fprintf(w, `{"data":[%s],"links":{"next":null}}`,
				fileItem("atb.assembly.202505_all.batch.0002.agc"))
		case "/metadata":
			w.Write(gzipTSV(t, "batch_name\told_name\n"+
				"atb.assembly.202505_all.batch.0001\tEscherichia_coli_global_ordered.part001\n"+
				"atb.assembly.202505_all.batch.0002\tunknown.part002\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	return srv
}

func TestBuildAGCCollectionIndex(t *testing.T) {
	srv := buildJoinServer(t)
	defer srv.Close()

	rootURLFor := func(nodeID string) string { return srv.URL + "/" + nodeID + "/root/" }
	nodes := []sources.AGCNode{{ID: "nodeA"}, {ID: "nodeB"}}

	idx, unmatched, err := BuildAGCCollectionIndex(rootURLFor, nodes, srv.URL+"/metadata")
	if err != nil {
		t.Fatalf("BuildAGCCollectionIndex: %v", err)
	}

	species := map[string]string{}
	for _, e := range idx.Entries {
		species[e.Filename] = e.Project
	}
	if got := species["atb.assembly.202505_all.batch.0001.agc"]; got != "Escherichia_coli" {
		t.Errorf("batch.0001 species: got %q, want Escherichia_coli", got)
	}
	if got := species["atb.assembly.202505_all.batch.0002.agc"]; got != "unknown" {
		t.Errorf("batch.0002 species: got %q, want unknown", got)
	}
	if got := species["atb.assembly.202505_all.batch.0999.agc"]; got != "" {
		t.Errorf("batch.0999 has no metadata; species must be empty, got %q", got)
	}
	if len(unmatched) != 1 || unmatched[0] != "atb.assembly.202505_all.batch.0999.agc" {
		t.Errorf("unmatched: got %v, want [batch.0999.agc]", unmatched)
	}
}

func TestCrawlAGCCollectionLeavesProjectEmpty(t *testing.T) {
	srv := buildJoinServer(t)
	defer srv.Close()
	rootURLFor := func(nodeID string) string { return srv.URL + "/" + nodeID + "/root/" }

	idx, err := CrawlAGCCollection(rootURLFor, []sources.AGCNode{{ID: "nodeA"}})
	if err != nil {
		t.Fatalf("CrawlAGCCollection: %v", err)
	}
	if len(idx.Entries) == 0 {
		t.Fatal("no entries crawled")
	}
	for _, e := range idx.Entries {
		if e.Project != "" {
			t.Errorf("%s: Project = %q, want empty (species comes from the join)", e.Filename, e.Project)
		}
	}
}

func TestFetchAGCCollectionJoinsSpecies(t *testing.T) {
	srv := buildJoinServer(t)
	defer srv.Close()
	dir := t.TempDir()
	rootURLFor := func(nodeID string) string { return srv.URL + "/" + nodeID + "/root/" }
	nodes := []sources.AGCNode{{ID: "nodeA"}, {ID: "nodeB"}}

	idx, err := FetchAGCCollection(dir, rootURLFor, nodes, srv.URL+"/metadata", true)
	if err != nil {
		t.Fatalf("FetchAGCCollection: %v", err)
	}
	found := false
	for _, e := range idx.Entries {
		if e.Filename == "atb.assembly.202505_all.batch.0001.agc" && e.Project == "Escherichia_coli" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected batch.0001 joined to Escherichia_coli; entries=%+v", idx.Entries)
	}
	// The cached TSV must exist and re-parse (tolerant of the one unmatched batch).
	if _, err := os.Stat(filepath.Join(dir, sources.AGCIndexFilename)); err != nil {
		t.Errorf("cache not written: %v", err)
	}
}
