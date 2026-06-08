// Package agc reads AGC (Assembled Genomes Compressor) archives by shelling
// out to the upstream `agc` binary. atb fetches and caches that binary next to
// itself; agc is never linked into atb, so atb stays pure Go and statically
// linked. The package is read-only: it lists samples/contigs and extracts
// FASTA, but never creates or modifies archives.
package agc

import (
	"fmt"
	"runtime"

	"github.com/allthebacteria/atb-cli/internal/sources"
)

// binaryName returns the agc executable name for the host OS.
func binaryName() string {
	if runtime.GOOS == "windows" {
		return "agc.exe"
	}
	return "agc"
}

// platformAsset returns the agc release-asset filename for the given
// GOOS/GOARCH and reports whether that asset is a zip (Windows) rather than a
// tar.gz. It errors on platforms for which agc publishes no binary.
func platformAsset(goos, goarch string) (asset string, isZip bool, err error) {
	type key struct{ os, arch string }
	switch (key{goos, goarch}) {
	case key{"linux", "amd64"}:
		return fmt.Sprintf("agc-%s-x64_linux.tar.gz", sources.AGCVersion), false, nil
	case key{"linux", "arm64"}:
		return fmt.Sprintf("agc-%s-arm64_linux.tar.gz", sources.AGCVersion), false, nil
	case key{"darwin", "amd64"}:
		return fmt.Sprintf("agc-%s-x64_mac.tar.gz", sources.AGCVersion), false, nil
	case key{"darwin", "arm64"}:
		return fmt.Sprintf("agc-%s-m1_mac.tar.gz", sources.AGCVersion), false, nil
	case key{"windows", "amd64"}:
		return fmt.Sprintf("agc-%s-x64_windows.zip", sources.AGCVersion), true, nil
	case key{"windows", "arm64"}:
		return "", false, fmt.Errorf("agc publishes no windows/arm64 binary; run the x64 build under emulation or use WSL")
	default:
		return "", false, fmt.Errorf("unsupported platform %s/%s for agc", goos, goarch)
	}
}
