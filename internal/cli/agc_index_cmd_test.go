package cli

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/allthebacteria/atb-cli/internal/sources"
)

// gzipTSV gzips s for a metadata endpoint. Copied from the osf test package
// because Go test helpers do not cross packages.
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
// A holds batch.0001 (metadata match) and batch.0999 (no metadata, so unmatched);
// node B holds batch.0002. Copied verbatim from the osf test package.
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

func TestRunAGCIndexJoinsAndWrites(t *testing.T) {
	srv := buildJoinServer(t) // reuse the osf-style two-node+metadata server shape
	defer srv.Close()
	rootURLFor := func(nodeID string) string { return srv.URL + "/" + nodeID + "/root/" }
	nodes := []sources.AGCNode{{ID: "nodeA"}, {ID: "nodeB"}}

	var buf bytes.Buffer
	// batch.0999 has no metadata, so it is unmatched: fail closed, nothing written.
	_, unmatched, err := runAGCIndex(&buf, rootURLFor, nodes, srv.URL+"/metadata")
	if err == nil {
		t.Fatalf("expected fail-closed error on unmatched batch")
	}
	if unmatched != 1 {
		t.Errorf("unmatched: got %d, want 1", unmatched)
	}
	if buf.Len() != 0 {
		t.Errorf("index must not be written when a batch is unmatched; got %d bytes", buf.Len())
	}
}

func TestWriteAGCIndexKeepsOutputOnFailClose(t *testing.T) {
	srv := buildJoinServer(t) // batch.0999 has no metadata, so the crawl fails closed
	defer srv.Close()
	rootURLFor := func(nodeID string) string { return srv.URL + "/" + nodeID + "/root/" }
	nodes := []sources.AGCNode{{ID: "nodeA"}, {ID: "nodeB"}}

	const sentinel = "PRE-EXISTING INDEX\n"
	path := filepath.Join(t.TempDir(), "atb_agc_files.tsv")
	if err := os.WriteFile(path, []byte(sentinel), 0o644); err != nil {
		t.Fatalf("seed output file: %v", err)
	}

	// A fail-closed crawl must leave the pre-existing file untouched, not truncate it.
	if _, err := writeAGCIndexOutput(path, io.Discard, rootURLFor, nodes, srv.URL+"/metadata"); err == nil {
		t.Fatalf("expected fail-closed error on unmatched batch")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if string(got) != sentinel {
		t.Errorf("output file changed on fail-closed run: got %q, want %q", string(got), sentinel)
	}
}

func TestWriteAGCIndexFailsClosedOnDuplicateBatch(t *testing.T) {
	// One node's agc_batches listing returns the same batch filename twice - the
	// symptom of inconsistent OSF pagination. The batch has metadata, so it clears
	// the unmatched check and is caught only by the duplicate guard. The publish
	// path must fail closed and write no file.
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
			// The same batch is listed twice.
			fmt.Fprintf(w, `{"data":[%s,%s],"links":{"next":null}}`,
				fileItem("atb.assembly.202505_all.batch.0001.agc"),
				fileItem("atb.assembly.202505_all.batch.0001.agc"))
		case "/metadata":
			w.Write(gzipTSV(t, "batch_name\told_name\n"+
				"atb.assembly.202505_all.batch.0001\tEscherichia_coli_global_ordered.part001\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	rootURLFor := func(nodeID string) string { return srv.URL + "/" + nodeID + "/root/" }
	nodes := []sources.AGCNode{{ID: "nodeA"}}

	out := filepath.Join(t.TempDir(), "atb_agc_files.tsv")
	if _, err := writeAGCIndexOutput(out, io.Discard, rootURLFor, nodes, srv.URL+"/metadata"); err == nil {
		t.Fatalf("expected fail-closed error on a duplicate batch")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("output file must not be created on a duplicate batch; os.Stat err = %v", err)
	}
}

func TestRunAGCIndexWritesJoinedTSV(t *testing.T) {
	// A fully-matched sibling of buildJoinServer: every crawled batch is present
	// in the metadata, so the join leaves nothing unmatched and the index writes.
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
			fmt.Fprintf(w, `{"data":[%s],"links":{"next":null}}`,
				fileItem("atb.assembly.202505_all.batch.0001.agc"))
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
	defer srv.Close()

	rootURLFor := func(nodeID string) string { return srv.URL + "/" + nodeID + "/root/" }
	nodes := []sources.AGCNode{{ID: "nodeA"}, {ID: "nodeB"}}

	var buf bytes.Buffer
	written, unmatched, err := runAGCIndex(&buf, rootURLFor, nodes, srv.URL+"/metadata")
	if err != nil {
		t.Fatalf("runAGCIndex: %v", err)
	}
	if unmatched != 0 {
		t.Errorf("unmatched: got %d, want 0", unmatched)
	}
	if written != 2 {
		t.Errorf("written: got %d, want 2", written)
	}
	out := buf.String()
	// 6-column header so osf.ParseIndex round-trips the file.
	if !strings.HasPrefix(out, "project\tproject_id\tfilename\turl\tmd5\tsize_mb") {
		t.Errorf("output missing TSV header, got first line:\n%s", strings.SplitN(out, "\n", 2)[0])
	}
	for _, want := range []string{
		"Escherichia_coli",
		"atb.assembly.202505_all.batch.0001.agc",
		"unknown",
		"atb.assembly.202505_all.batch.0002.agc",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestAGCIndexHelp(t *testing.T) {
	stdout, _, err := runCmd("agc", "index", "--help")
	if err != nil {
		t.Fatalf("agc index --help: %v", err)
	}
	for _, want := range []string{"--output"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected %q in agc index --help, got:\n%s", want, stdout)
		}
	}
}
