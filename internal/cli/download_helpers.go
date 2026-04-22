package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/allthebacteria/atb-cli/internal/download"
	"github.com/allthebacteria/atb-cli/internal/sources"
)

// AssemblyDownloadConfig holds options for downloading genome assemblies.
type AssemblyDownloadConfig struct {
	SampleAccessions []string
	OutputDir        string
	Parallel         int
	DryRun           bool
	MaxSamples       int
	Force            bool
	MinFreeSpaceGB   int
	Stderr           io.Writer
}

// downloadAssemblies deduplicates accessions, constructs S3 URLs, and downloads
// the corresponding FASTA assemblies. It prints progress and summary to stderr.
func downloadAssemblies(cfg AssemblyDownloadConfig) error {
	w := cfg.Stderr
	if w == nil {
		w = os.Stderr
	}

	accessions := deduplicateAccessions(cfg.SampleAccessions)
	if len(accessions) == 0 {
		return nil
	}

	if cfg.MaxSamples > 0 && len(accessions) > cfg.MaxSamples {
		fmt.Fprintf(w, "Capping to %d assemblies (--max-samples)\n", cfg.MaxSamples)
		accessions = accessions[:cfg.MaxSamples]
	}

	urls := make([]string, len(accessions))
	for i, acc := range accessions {
		urls[i] = buildAssemblyURL(acc)
	}

	if cfg.DryRun {
		fmt.Fprintf(w, "Would download %d assembly file(s) to %s\n", len(urls), cfg.OutputDir)
		for _, u := range urls {
			fmt.Fprintf(w, "  %s\n", u)
		}
		return nil
	}

	fmt.Fprintf(w, "Downloading %d assembly file(s) to %s\n", len(urls), cfg.OutputDir)

	dl := download.New(download.Config{
		OutputDir:      cfg.OutputDir,
		Parallel:       cfg.Parallel,
		CheckDiskSpace: !cfg.Force,
		MinFreeSpaceGB: cfg.MinFreeSpaceGB,
		MaxRetries:     3,
	})

	result := dl.DownloadAll(urls)

	fmt.Fprintf(w, "Completed: %d/%d  Failed: %d  Bytes: %s\n",
		result.Completed, result.Total, result.Failed, formatSize(result.Bytes))

	for _, e := range result.Errors {
		fmt.Fprintf(w, "  error: %s: %s\n", e.URL, e.Error)
	}

	if err := dl.WriteManifest("atb assembly download", result); err != nil {
		fmt.Fprintf(w, "warning: failed to write manifest: %v\n", err)
	}

	if result.Failed > 0 {
		return fmt.Errorf("%d download(s) failed", result.Failed)
	}

	return nil
}

// deduplicateAccessions removes duplicate accessions while preserving order.
func deduplicateAccessions(accessions []string) []string {
	seen := make(map[string]struct{}, len(accessions))
	out := make([]string, 0, len(accessions))
	for _, a := range accessions {
		if _, ok := seen[a]; !ok {
			seen[a] = struct{}{}
			out = append(out, a)
		}
	}
	return out
}

// buildAssemblyURL constructs the S3 download URL for a genome assembly.
func buildAssemblyURL(sampleAccession string) string {
	return sources.AssemblyBaseURL + sampleAccession + ".fa.gz"
}
