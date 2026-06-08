package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/allthebacteria/atb-cli/internal/agc"
)

func newAGCCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agc",
		Short: "Read sequences from AGC genome archives",
		Long: `Read AGC (Assembled Genomes Compressor) archives: list samples and
contigs, and extract sequences as FASTA.

Uses the upstream 'agc' binary. Run 'atb agc install' to download it
(Linux, macOS, and Windows x64).`,
	}
	cmd.AddCommand(newAGCInstallCmd())
	cmd.AddCommand(newAGCListCmd())
	cmd.AddCommand(newAGCInfoCmd())
	cmd.AddCommand(newAGCGetCmd())
	return cmd
}

func newAGCInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "install",
		Short:   "Download the agc binary (Linux/macOS/Windows-x64)",
		Example: `  atb agc install`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if path, err := agc.FindBinary(); err == nil {
				fmt.Fprintf(os.Stderr, "agc already installed at %s\n", path)
				return nil
			}
			return agc.InstallBinary(func(msg string) {
				fmt.Fprintln(os.Stderr, msg)
			})
		},
	}
}

func newAGCListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls <file.agc> [sample]",
		Short: "List samples in an archive, or contigs in a sample",
		Long: `With one argument, list the sample names in the archive.
With two arguments, list the contig names within the given sample.`,
		Example: `  atb agc ls genomes.agc
  atb agc ls genomes.agc SAMD00000344`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := agc.FindBinary(); err != nil {
				return err
			}
			var (
				names []string
				err   error
			)
			if len(args) == 1 {
				names, err = agc.ListSamples(args[0])
			} else {
				names, err = agc.ListContigs(args[0], args[1])
			}
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			for _, n := range names {
				fmt.Fprintln(w, n)
			}
			return nil
		},
	}
}

func newAGCInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "info <file.agc>",
		Short:   "Show archive metadata",
		Example: `  atb agc info genomes.agc`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := agc.FindBinary(); err != nil {
				return err
			}
			text, err := agc.Info(args[0])
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), text)
			return nil
		},
	}
}

func newAGCGetCmd() *cobra.Command {
	var (
		output     string
		threads    int
		lineLength int
		gzipLevel  int
		streaming  bool
		samples    []string
		all        bool
	)

	cmd := &cobra.Command{
		Use:   "get <file.agc> [contig...]",
		Short: "Extract sequences as FASTA",
		Long: `Extract sequences from an AGC archive as FASTA.

Three mutually exclusive selections:
  - one or more contig queries (positional): contig[@sample][:from-to]
  - --sample: extract whole sample(s)
  - --all:    extract the entire collection

Output goes to stdout unless -o is given.`,
		Example: `  # One contig region to stdout
  atb agc get genomes.agc "contig_1@SAMD00000344:1000-2000"

  # Whole samples to a file
  atb agc get genomes.agc --sample SAMD00000344 --sample SAMD00000345 -o out.fa

  # Entire collection, gzip level 6, 8 threads
  atb agc get genomes.agc --all --gzip 6 -t 8 -o all.fa.gz`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			archive := args[0]
			contigs := args[1:]

			if all && (len(samples) > 0 || len(contigs) > 0) {
				return fmt.Errorf("--all cannot be combined with --sample or contig arguments")
			}
			if len(samples) > 0 && len(contigs) > 0 {
				return fmt.Errorf("--sample cannot be combined with contig arguments")
			}
			if !all && len(samples) == 0 && len(contigs) == 0 {
				return fmt.Errorf("specify one or more contigs, --sample, or --all")
			}

			if _, err := agc.FindBinary(); err != nil {
				return err
			}

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

			opts := agc.Options{
				Threads:    threads,
				LineLength: lineLength,
				GzipLevel:  gzipLevel,
				Streaming:  streaming,
			}

			var runErr error
			switch {
			case all:
				runErr = agc.GetCollection(archive, w, opts)
			case len(samples) > 0:
				runErr = agc.GetSamples(archive, samples, w, opts)
			default:
				runErr = agc.GetContigs(archive, contigs, w, opts)
			}
			if out != nil {
				if cerr := out.Close(); cerr != nil && runErr == nil {
					runErr = fmt.Errorf("close output file: %w", cerr)
				}
			}
			return runErr
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "write FASTA to this file (default stdout)")
	cmd.Flags().IntVarP(&threads, "threads", "t", 0, "CPU threads (default: all cores minus one)")
	cmd.Flags().IntVarP(&lineLength, "line-length", "l", 0, "FASTA line wrap length (default: agc's 80)")
	cmd.Flags().IntVar(&gzipLevel, "gzip", 0, "gzip the output at this level (0 = uncompressed)")
	cmd.Flags().BoolVarP(&streaming, "streaming", "s", false, "streaming mode: slower, lower memory (ignored with --all)")
	cmd.Flags().StringSliceVar(&samples, "sample", nil, "extract whole sample(s) by name (repeatable)")
	cmd.Flags().BoolVar(&all, "all", false, "extract the entire collection")

	return cmd
}
