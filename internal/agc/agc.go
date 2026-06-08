// Package agc reads AGC (Assembled Genomes Compressor) archives by shelling
// out to the upstream `agc` binary. atb fetches and caches that binary next to
// itself; agc is never linked into atb, so atb stays pure Go and statically
// linked. The package is read-only: it lists samples/contigs and extracts
// FASTA, but never creates or modifies archives.
package agc

import (
	"runtime"
)

// binaryName returns the agc executable name for the host OS.
func binaryName() string {
	if runtime.GOOS == "windows" {
		return "agc.exe"
	}
	return "agc"
}
