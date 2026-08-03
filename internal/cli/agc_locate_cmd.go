package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/allthebacteria/atb-cli/internal/agc"
	"github.com/allthebacteria/atb-cli/internal/sources"
)

func newAGCLocateCmd() *cobra.Command {
	var (
		fromFile string
		format   string
		refresh  bool
	)

	cmd := &cobra.Command{
		Use:   "locate [accession...]",
		Short: "Look up which AGC batch (species and OSF node) holds each accession",
		Long: `Resolve sample accessions to the AGC batch that contains them, without
downloading anything. This is the search half of 'atb agc download': it answers
"which batch is my sample in, and is that batch available yet?".

Accessions come from positional arguments, a --from file (a query result with a
sample_accession column, or one accession per line; - for stdin), or piped stdin.
The accession->batch map and the batch index are fetched and cached exactly as
'atb agc download' fetches them; no agc binary is required.`,
		Example: `  # One accession
  atb agc locate SAMEA2247573

  # A whole query result, as JSON for a pipeline
  atb query --species "Escherichia coli" --limit 5 --format tsv | \
    atb agc locate --from - --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "tsv" && format != "json" {
				return fmt.Errorf("--format must be tsv or json, got %q", format)
			}
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			dataDir := resolveDataDir(cfg)
			cacheDir := agc.ArchiveDir(dataDir, cfg.AGC.ArchiveDir)

			accessions := append([]string{}, args...)
			if fromFile != "" {
				fromAcc, err := readAccessionsFromFile(fromFile)
				if err != nil {
					return fmt.Errorf("reading --from: %w", err)
				}
				accessions = append(accessions, fromAcc...)
			}
			if len(accessions) == 0 {
				if fi, statErr := os.Stdin.Stat(); statErr == nil && (fi.Mode()&os.ModeCharDevice) == 0 {
					piped, err := readAccessionsFromFile("-")
					if err != nil {
						return err
					}
					accessions = append(accessions, piped...)
				}
			}
			accessions = dedupeStrings(accessions)
			if len(accessions) == 0 {
				return fmt.Errorf("no accessions given; pass them as arguments, via --from <file>, or pipe them to stdin")
			}

			m, err := agc.FetchMap(cacheDir, cfg.AGC.ArchiveMapURL, refresh)
			if err != nil {
				return err
			}
			idx, err := loadAGCBatchIndex("", sources.AGCIndexURL, cacheDir, refresh)
			if err != nil {
				return err
			}
			byName := agc.RefsFromIndex(idx)

			results := agc.Locate(accessions, m, byName)
			w := cmd.OutOrStdout()
			if format == "json" {
				return writeLocateJSON(w, results)
			}
			return writeLocateTSV(w, results)
		},
	}

	cmd.Flags().StringVar(&fromFile, "from", "", "CSV/TSV with a sample_accession column, or one accession per line (- for stdin)")
	cmd.Flags().StringVar(&format, "format", "tsv", "output format: tsv or json")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "re-fetch the accession->batch map and the batch index even if cached")

	return cmd
}

// locateDisplay maps a result's status to its batch/species/node cells, using
// placeholder tokens for the two not-found cases so every row is unambiguous.
func locateDisplay(r agc.LocateResult) (batch, species, node string) {
	switch r.Status {
	case agc.LocateUnresolved:
		return "<unresolved>", "<unresolved>", "<unresolved>"
	case agc.LocateNotYetAvailable:
		return r.Batch, "<not-yet-available>", "<not-yet-available>"
	default:
		return r.Batch, r.Species, r.Node
	}
}

func writeLocateTSV(w io.Writer, results []agc.LocateResult) error {
	bw := bufio.NewWriter(w)
	if _, err := fmt.Fprintln(bw, "accession\tbatch\tspecies\tnode"); err != nil {
		return err
	}
	for _, r := range results {
		batch, species, node := locateDisplay(r)
		if _, err := fmt.Fprintf(bw, "%s\t%s\t%s\t%s\n", r.Accession, batch, species, node); err != nil {
			return err
		}
	}
	return bw.Flush()
}

// locateJSONRow is the JSON shape for one locate result: it adds the resolved
// OSF url that the TSV omits.
type locateJSONRow struct {
	Accession string `json:"accession"`
	Batch     string `json:"batch"`
	Species   string `json:"species"`
	Node      string `json:"node"`
	URL       string `json:"url"`
}

func writeLocateJSON(w io.Writer, results []agc.LocateResult) error {
	rows := make([]locateJSONRow, 0, len(results))
	for _, r := range results {
		batch, species, node := locateDisplay(r)
		rows = append(rows, locateJSONRow{Accession: r.Accession, Batch: batch, Species: species, Node: node, URL: r.URL})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}
