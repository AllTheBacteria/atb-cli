package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/allthebacteria/atb-cli/internal/osf"
	"github.com/allthebacteria/atb-cli/internal/sources"
)

// runAGCIndex crawls an OSF node's agc_batches/ folder and writes the resulting
// by-species index as a 6-column TSV to w, returning the number of batch entries
// written. rootURL is the node's osfstorage listing (build it with
// sources.OSFNodeFilesURL); node is stamped onto every row's project_id. This is
// the network+format seam behind `atb agc index`, kept apart from the cobra glue
// so it is unit-testable against a local server with no config plumbing.
func runAGCIndex(w io.Writer, rootURL, node string) (int, error) {
	idx, err := osf.CrawlAGCIndex(rootURL, node)
	if err != nil {
		return 0, err
	}
	if err := osf.WriteAGCIndexTSV(idx, w); err != nil {
		return 0, err
	}
	return len(idx.Entries), nil
}

func newAGCIndexCmd() *cobra.Command {
	var (
		output  string
		osfNode string
	)

	cmd := &cobra.Command{
		Use:   "index",
		Short: "Crawl the OSF node's AGC batches into a searchable TSV index",
		Long: `Crawl an OSF node's agc_batches/ folder and write a separate AGC index
(atb_agc_files.tsv): one row per .agc batch with its species, OSF download URL,
md5, and size. This is the index that 'atb agc download --species' searches to
decide which batches to download — generate it once and commit it for offline use
(pass it back via --agc-index), or let 'atb agc download' crawl and cache it on demand.

The index is a 6-column TSV (project, project_id, filename, url, md5, size_mb) —
the same layout as the master OSF index, so the existing parser round-trips it.`,
		Example: `  # Write the index to a file you can commit
  atb agc index -o atb_agc_files.tsv

  # Print it to stdout
  atb agc index`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			node := resolveOSFNode(osfNode, cfg.AGC.OSFNode)

			w := cmd.OutOrStdout()
			var out *os.File
			if output != "" {
				f, err := os.Create(output)
				if err != nil {
					return fmt.Errorf("create output file: %w", err)
				}
				out = f
				w = f
			}

			n, runErr := runAGCIndex(w, sources.OSFNodeFilesURL(node), node)
			if out != nil {
				if cerr := out.Close(); cerr != nil && runErr == nil {
					runErr = fmt.Errorf("close output file: %w", cerr)
				}
			}
			if runErr != nil {
				return runErr
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Wrote %d AGC batch(es) from OSF node %s\n", n, node)
			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "write the index TSV to this file (default stdout)")
	cmd.Flags().StringVar(&osfNode, "osf-node", "", "OSF node to crawl (default from config or the staging node)")

	return cmd
}
