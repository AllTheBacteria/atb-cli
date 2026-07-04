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

	urls, err := parseCSVURLs(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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

	urls, err := parseCSVURLs(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "https://allthebacteria-assemblies.s3.eu-west-2.amazonaws.com/SAMEA104027617.fa.gz"
	if len(urls) != 1 || urls[0] != want {
		t.Fatalf("expected [%q], got %v", want, urls)
	}
}
