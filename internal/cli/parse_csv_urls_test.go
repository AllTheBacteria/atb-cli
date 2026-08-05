package cli

import "testing"

// Issue #27: `atb mlst ... --download` and `atb download --from <mlst output>`
// failed because the mlst result file has a sample_accession column but no
// aws_url column. parseCSVURLs returned the bare accession, which the
// downloader rejected with "unsupported protocol scheme". A scheme-less value
// must be converted into a full S3 assembly URL.
func TestParseCSVURLsBuildsURLFromAccession(t *testing.T) {
	content := "sample_accession\tsylph_species\tmlst_st\n" +
		"SAMEA104027617\tEscherichia coli\t131\n" +
		"SAMD00000355\tSalmonella enterica\t11\n"
	path := writeTempFile(t, "mlst-*.tsv", content)

	urls, skipped, err := parseCSVURLs(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skipped != 0 {
		t.Errorf("skipped: got %d, want 0", skipped)
	}

	want := []string{
		"https://allthebacteria-assemblies.s3.eu-west-2.amazonaws.com/SAMEA104027617.fa.gz",
		"https://allthebacteria-assemblies.s3.eu-west-2.amazonaws.com/SAMD00000355.fa.gz",
	}
	if len(urls) != len(want) {
		t.Fatalf("expected %d URLs, got %d: %v", len(want), len(urls), urls)
	}
	for i, w := range want {
		if urls[i] != w {
			t.Errorf("urls[%d]: got %q, want %q", i, urls[i], w)
		}
	}
}

// A file that already carries a full aws_url column must pass through unchanged
// so the fix does not double-wrap URLs that are already valid.
func TestParseCSVURLsPassesThroughAWSURL(t *testing.T) {
	content := "sample_accession\taws_url\n" +
		"SAMEA104027617\thttps://allthebacteria-assemblies.s3.eu-west-2.amazonaws.com/SAMEA104027617.fa.gz\n"
	path := writeTempFile(t, "results-*.tsv", content)

	urls, skipped, err := parseCSVURLs(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skipped != 0 {
		t.Errorf("skipped: got %d, want 0", skipped)
	}

	want := "https://allthebacteria-assemblies.s3.eu-west-2.amazonaws.com/SAMEA104027617.fa.gz"
	if len(urls) != 1 || urls[0] != want {
		t.Fatalf("expected [%q], got %v", want, urls)
	}
}

// The index records a sample with no assembly as the literal string NA in
// aws_url, on 464,368 of its rows. Treating that as a value produced
// ".../NA.fa.gz" and a download attempt per row. Those samples have no
// assembly on S3 at all, so the row is skipped and counted.
func TestParseCSVURLsSkipsRowsWithNoAssembly(t *testing.T) {
	content := "sample_accession\tdataset\taws_url\n" +
		"SAMEA103931393\tnot_processed\tNA\n" +
		"SAMEA10029803\tr0.2\thttps://allthebacteria-assemblies.s3.eu-west-2.amazonaws.com/SAMEA10029803.fa.gz\n" +
		"SAMEA104156871\treject\tNA\n"
	path := writeTempFile(t, "results-*.tsv", content)

	urls, skipped, err := parseCSVURLs(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"https://allthebacteria-assemblies.s3.eu-west-2.amazonaws.com/SAMEA10029803.fa.gz"}
	if len(urls) != len(want) || urls[0] != want[0] {
		t.Fatalf("urls: got %v, want %v", urls, want)
	}
	if skipped != 2 {
		t.Errorf("skipped: got %d, want 2", skipped)
	}
}

// A file whose aws_url column is NA throughout yields nothing to download.
// The caller must be able to tell that apart from an empty file.
func TestParseCSVURLsCountsAnAllNAFile(t *testing.T) {
	content := "sample_accession\taws_url\nSAMEA103931393\tNA\nSAMEA104156871\tna\n"
	path := writeTempFile(t, "results-*.tsv", content)

	urls, skipped, err := parseCSVURLs(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(urls) != 0 {
		t.Errorf("urls: got %v, want none", urls)
	}
	if skipped != 2 {
		t.Errorf("skipped: got %d, want 2", skipped)
	}
}

// The NA sentinel also appears in a sample_accession column when a result file
// carries no aws_url column at all.
func TestParseCSVURLsSkipsNAInTheAccessionFallback(t *testing.T) {
	content := "sample_accession\tmlst_st\nNA\t131\nSAMD00000355\t11\n"
	path := writeTempFile(t, "mlst-*.tsv", content)

	urls, skipped, err := parseCSVURLs(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "https://allthebacteria-assemblies.s3.eu-west-2.amazonaws.com/SAMD00000355.fa.gz"
	if len(urls) != 1 || urls[0] != want {
		t.Fatalf("urls: got %v, want [%q]", urls, want)
	}
	if skipped != 1 {
		t.Errorf("skipped: got %d, want 1", skipped)
	}
}
