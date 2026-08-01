package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/allthebacteria/atb-cli/internal/agc"
)

func TestAGCLocateTSVColumns(t *testing.T) {
	results := []agc.LocateResult{
		{Accession: "SAMEA1", Batch: "b1", Species: "Escherichia_coli", Node: "4jq8u", URL: "u1", Status: agc.LocateFound},
		{Accession: "SAMEA2", Batch: "b2", Status: agc.LocateNotYetAvailable},
		{Accession: "SAMEA3", Status: agc.LocateUnresolved},
	}
	var buf bytes.Buffer
	if err := writeLocateTSV(&buf, results); err != nil {
		t.Fatalf("writeLocateTSV: %v", err)
	}
	got := buf.String()
	want := "accession\tbatch\tspecies\tnode\n" +
		"SAMEA1\tb1\tEscherichia_coli\t4jq8u\n" +
		"SAMEA2\tb2\t<not-yet-available>\t<not-yet-available>\n" +
		"SAMEA3\t<unresolved>\t<unresolved>\t<unresolved>\n"
	if got != want {
		t.Errorf("TSV mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestAGCLocateTSV(t *testing.T) {
	dir := t.TempDir()
	seedArchiveMap(t, dir, "ACC1 Escherichia_coli_global_ordered_0001\nACC2 Ghost_global_ordered_9999\n")
	seedAGCCollectionIndex(t, dir, agcCollectionTSV) // lists the E. coli batch on jmeqg

	stdout, _, err := runCmd("agc", "locate", "--data-dir", dir, "ACC1", "ACC2", "ACC3")
	if err != nil {
		t.Fatalf("agc locate: %v", err)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if lines[0] != "accession\tbatch\tspecies\tnode" {
		t.Errorf("header = %q", lines[0])
	}
	if !strings.Contains(stdout, "ACC1\tEscherichia_coli_global_ordered_0001\tEscherichia_coli\tjmeqg") {
		t.Errorf("ACC1 found row wrong:\n%s", stdout)
	}
	if !strings.Contains(stdout, "ACC2\tGhost_global_ordered_9999\t<not-yet-available>\t<not-yet-available>") {
		t.Errorf("ACC2 not-yet row wrong:\n%s", stdout)
	}
	if !strings.Contains(stdout, "ACC3\t<unresolved>\t<unresolved>\t<unresolved>") {
		t.Errorf("ACC3 unresolved row wrong:\n%s", stdout)
	}
}

func TestAGCLocateJSON(t *testing.T) {
	dir := t.TempDir()
	seedArchiveMap(t, dir, "ACC1 Escherichia_coli_global_ordered_0001\n")
	seedAGCCollectionIndex(t, dir, agcCollectionTSV)

	stdout, _, err := runCmd("agc", "locate", "--data-dir", dir, "--format", "json", "ACC1")
	if err != nil {
		t.Fatalf("agc locate --format json: %v", err)
	}
	var rows []struct{ Accession, Batch, Species, Node, URL string }
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(rows) != 1 || rows[0].Accession != "ACC1" || rows[0].URL != "https://osf.io/download/ec1/" {
		t.Errorf("json rows wrong: %+v", rows)
	}
	if rows[0].Species != "Escherichia_coli" || rows[0].Node != "jmeqg" {
		t.Errorf("json species/node wrong: %+v", rows[0])
	}
}

func TestAGCLocateFromFile(t *testing.T) {
	dir := t.TempDir()
	seedArchiveMap(t, dir, "ACC1 Escherichia_coli_global_ordered_0001\n")
	seedAGCCollectionIndex(t, dir, agcCollectionTSV)

	from := filepath.Join(t.TempDir(), "ids.txt")
	if err := os.WriteFile(from, []byte("ACC1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := runCmd("agc", "locate", "--data-dir", dir, "--from", from)
	if err != nil {
		t.Fatalf("agc locate --from: %v", err)
	}
	if !strings.Contains(stdout, "ACC1\tEscherichia_coli_global_ordered_0001\tEscherichia_coli\tjmeqg") {
		t.Errorf("--from row wrong:\n%s", stdout)
	}
}

func TestAGCLocateNoAccessions(t *testing.T) {
	_, _, err := runCmd("agc", "locate", "--data-dir", t.TempDir())
	if err == nil {
		t.Fatal("expected an error when no accessions are given")
	}
}
