package cli

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/allthebacteria/atb-cli/internal/sources"
)

// resolveOSFNode picks the OSF node id for a by-species (Mode A) crawl:
// the --osf-node flag wins, then the configured agc.osf_node, and finally the
// staging-node default in sources. The staging id ("z7q5y") therefore appears
// only in sources, never in command logic, so promotion to production is a
// config edit (ADR §2.2/§4.9).
func resolveOSFNode(flag, cfgNode string) string {
	return firstNonEmpty(flag, cfgNode, sources.AGCTestNodeID)
}

// readAccessionsFromFile reads sample accessions from a CSV/TSV file (using the
// sample_accession column when a header is present) or, for a headerless file,
// the first whitespace-delimited token of each non-empty, non-comment line.
// A path of "-" reads from stdin.
func readAccessionsFromFile(path string) ([]string, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
	} else {
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, err
		}
	}
	return parseAccessions(string(data))
}

func parseAccessions(text string) ([]string, error) {
	firstLine := text
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		firstLine = text[:i]
	}
	sep := ','
	if strings.Contains(firstLine, "\t") {
		sep = '\t'
	}
	if accessionHeaderIndex(firstLine, sep) >= 0 {
		return accessionColumn(text, sep)
	}
	return bareAccessions(text), nil
}

// accessionHeaderIndex returns the column index of a "sample_accession" header
// field, or -1 if the line is not such a header.
func accessionHeaderIndex(line string, sep rune) int {
	for i, h := range strings.Split(line, string(sep)) {
		if strings.ToLower(strings.TrimSpace(h)) == "sample_accession" {
			return i
		}
	}
	return -1
}

func accessionColumn(text string, sep rune) ([]string, error) {
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = sep
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}
	col := -1
	for i, h := range header {
		if strings.ToLower(strings.TrimSpace(h)) == "sample_accession" {
			col = i
			break
		}
	}
	if col == -1 {
		return nil, fmt.Errorf("no sample_accession column")
	}

	var out []string
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if col < len(rec) {
			if v := strings.TrimSpace(rec[col]); v != "" {
				out = append(out, v)
			}
		}
	}
	return out, nil
}

func bareAccessions(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, strings.Fields(line)[0])
	}
	return out
}

// dedupeStrings returns in with duplicates removed, preserving first-seen order.
func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	var out []string
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// countSamples sums the accessions across all archive groups.
func countSamples(groups map[string][]string) int {
	n := 0
	for _, accs := range groups {
		n += len(accs)
	}
	return n
}

// firstNonEmpty returns the first non-empty argument, or "".
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
