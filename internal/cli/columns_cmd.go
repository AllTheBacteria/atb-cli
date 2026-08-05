package cli

import (
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"

	cols "github.com/allthebacteria/atb-cli/internal/columns"
	"github.com/allthebacteria/atb-cli/internal/output"
)

// columnFields are the fields of the machine-readable listing.
var columnFields = []string{"name", "source", "in_index", "description"}

func newColumnsCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "columns",
		Short: "List the columns available to atb query --columns",
		Long: `List every column name accepted by atb query --columns, grouped by the table
it comes from.

Columns marked with * are not held in the SQLite index. Asking for one sends the
query to the parquet files, which is slower and needs those files downloaded.

This command reads no data, so it works before anything has been fetched.`,
		Example: `  # List every column
  atb columns

  # Just the names, for a script
  atb columns --format tsv | cut -f1 | tail -n +2

  # Use them
  atb query --species "Escherichia coli" --columns sample_accession,N50,Contamination`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			switch format {
			case "", "auto", "table":
				return printColumns(w)
			default:
				return output.Format(w, columnRows(), columnFields, format)
			}
		},
	}

	cmd.Flags().StringVar(&format, "format", "", "output format: table, tsv, csv, json")

	return cmd
}

func columnRows() []output.Row {
	rows := make([]output.Row, 0, len(cols.All()))
	for _, c := range cols.All() {
		rows = append(rows, output.Row{
			"name":        c.Name,
			"source":      string(c.Source),
			"in_index":    strconv.FormatBool(c.InIndex),
			"description": c.Description,
		})
	}
	return rows
}

// printColumns writes the listing grouped by source. Lines carrying no tab end
// the current alignment block, so each group lines up on its own widest name.
func printColumns(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	fmt.Fprintln(tw, "Columns accepted by atb query --columns.")

	var source cols.Source
	for _, c := range cols.All() {
		if c.Source != source {
			source = c.Source
			fmt.Fprintf(tw, "\n%s\n", source)
		}
		marker := " "
		if !c.InIndex {
			marker = "*"
		}
		fmt.Fprintf(tw, "  %s %s\t%s\n", marker, c.Name, c.Description)
	}

	fmt.Fprintf(tw, "\n%d columns. * marks one the SQLite index cannot answer: asking for it\n", len(cols.All()))
	fmt.Fprintln(tw, "sends the query to the parquet files, which is slower.")
	fmt.Fprintln(tw, "\nExample:")
	fmt.Fprintln(tw, `  atb query --species "Escherichia coli" --columns sample_accession,N50,Contamination`)

	return tw.Flush()
}
