package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/allthebacteria/atb-cli/internal/osf"
	"github.com/allthebacteria/atb-cli/internal/sources"
)

// runAGCIndex builds the combined collection index (crawl the nodes, join the
// batch metadata for the species column) and writes it as a 6-column TSV to w.
// It returns the number of rows written and the number of batches left without a
// species. It fails closed: if any batch is unmatched it writes nothing and
// returns an error, so a published index is never partial. rootURLFor, nodes, and
// metadataURL are parameters so the command is testable against a local server.
func runAGCIndex(w io.Writer, rootURLFor func(nodeID string) string, nodes []sources.AGCNode, metadataURL string) (written, unmatched int, err error) {
	idx, missing, err := osf.BuildAGCCollectionIndex(rootURLFor, nodes, metadataURL)
	if err != nil {
		return 0, 0, err
	}
	if len(missing) > 0 {
		return 0, len(missing), fmt.Errorf("%d batch(es) have no species in the metadata (first: %s); not writing a partial index", len(missing), missing[0])
	}
	if dup, ok := osf.FirstDuplicateBatch(idx); ok {
		return 0, 0, fmt.Errorf("crawl returned duplicate batch %q; the OSF listing may be paginating inconsistently", dup)
	}
	if err := osf.WriteAGCIndexTSV(idx, w); err != nil {
		return 0, 0, err
	}
	return len(idx.Entries), 0, nil
}

// writeAGCIndexOutput builds the combined index into a buffer and writes it out
// only after a successful crawl: to the output path when set, otherwise to
// stdout. A fail-closed crawl returns the error without creating or truncating
// the output file, so a pre-existing index survives a failed refresh.
func writeAGCIndexOutput(output string, stdout io.Writer, rootURLFor func(nodeID string) string, nodes []sources.AGCNode, metadataURL string) (written int, err error) {
	var buf bytes.Buffer
	n, _, err := runAGCIndex(&buf, rootURLFor, nodes, metadataURL)
	if err != nil {
		return 0, err
	}
	if output != "" {
		if err := os.WriteFile(output, buf.Bytes(), 0o644); err != nil {
			return 0, fmt.Errorf("write output file: %w", err)
		}
		return n, nil
	}
	if _, err := stdout.Write(buf.Bytes()); err != nil {
		return 0, err
	}
	return n, nil
}

func newAGCIndexCmd() *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "index",
		Short: "Crawl the OSF collection nodes and join metadata into a searchable TSV index",
		Long: `Crawl every OSF collection node's agc_batches/ folder and join the batch
metadata to write a separate AGC index (atb_agc_files.tsv): one row per .agc
batch with its species, OSF download URL, md5, and size. This is the index that
'atb agc download --species' searches to decide which batches to download -
generate it once and commit it for offline use (pass it back via --agc-index),
or let 'atb agc download' crawl and cache it on demand. It fails if any batch has
no species in the metadata, so a published index is never partial.

The index is a 6-column TSV (project, project_id, filename, url, md5, size_mb) -
the same layout as the master OSF index, so the existing parser round-trips it.`,
		Example: `  # Write the index to a file you can commit
  atb agc index -o atb_agc_files.tsv

  # Print it to stdout
  atb agc index`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := writeAGCIndexOutput(output, cmd.OutOrStdout(), sources.OSFNodeFilesURL, sources.AGCCollectionNodes, sources.AGCBatchMetadataURL)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Wrote %d AGC batch(es) from %d OSF node(s)\n", n, len(sources.AGCCollectionNodes))
			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "write the index TSV to this file (default stdout)")

	return cmd
}
