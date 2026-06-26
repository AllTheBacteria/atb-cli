package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Minimal OSF API fixtures: a node root whose single folder is agc_batches, and
// that folder's one-page file listing. Kept inline so the cli test is
// self-contained (the osf package owns the richer multi-page fixtures).
const agcNodeRootJSON = `{"data":[{"attributes":{"name":"agc_batches","kind":"folder"},` +
	`"relationships":{"files":{"links":{"related":{"href":"FOLDER_URL"}}}}}],"links":{"next":null}}`

const agcFolderJSON = `{"data":[` +
	`{"attributes":{"name":"Acinetobacter_baylyi_global_ordered_0001.agc","kind":"file","size":3890981,` +
	`"extra":{"hashes":{"md5":"7be632ec46828a45a4d6d01d77b8099d"}}},"links":{"download":"https://osf.io/download/aaa/"}},` +
	`{"attributes":{"name":"Streptococcus_suis_AA_global_ordered_0001.agc","kind":"file","size":10400000,` +
	`"extra":{"hashes":{"md5":"ccc"}}},"links":{"download":"https://osf.io/download/ccc/"}}` +
	`],"links":{"next":null}}`

func agcIndexTestServer() *httptest.Server {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/root":
			w.Write([]byte(strings.Replace(agcNodeRootJSON, "FOLDER_URL", server.URL+"/folder", 1)))
		case "/folder":
			w.Write([]byte(agcFolderJSON))
		default:
			http.NotFound(w, r)
		}
	}))
	return server
}

func TestRunAGCIndex(t *testing.T) {
	server := agcIndexTestServer()
	defer server.Close()

	var buf bytes.Buffer
	n, err := runAGCIndex(&buf, server.URL+"/root", "z7q5y")
	if err != nil {
		t.Fatalf("runAGCIndex: %v", err)
	}
	if n != 2 {
		t.Fatalf("count = %d, want 2", n)
	}
	out := buf.String()
	// 6-column header so osf.ParseIndex round-trips the file.
	if !strings.HasPrefix(out, "project\tproject_id\tfilename\turl\tmd5\tsize_mb") {
		t.Errorf("output missing TSV header, got first line:\n%s", strings.SplitN(out, "\n", 2)[0])
	}
	for _, want := range []string{
		"Acinetobacter_baylyi_global_ordered_0001.agc",
		"https://osf.io/download/aaa/",
		"7be632ec46828a45a4d6d01d77b8099d",
		"z7q5y",
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
	for _, want := range []string{"--output", "--osf-node"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected %q in agc index --help, got:\n%s", want, stdout)
		}
	}
}
