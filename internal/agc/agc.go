// Package agc reads AGC (Assembled Genomes Compressor) archives by shelling
// out to the upstream `agc` binary. atb fetches and caches that binary next to
// itself; agc is never linked into atb, so atb stays pure Go and statically
// linked. The package is read-only: it lists samples/contigs and extracts
// FASTA, but never creates or modifies archives.
package agc

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// FindBinary locates the agc binary. Search order:
//  1. The directory containing the running atb binary.
//  2. The system PATH.
//
// If neither has it, the returned error points at `atb agc install`.
func FindBinary() (string, error) {
	if atbPath, err := os.Executable(); err == nil {
		atbPath, _ = filepath.EvalSymlinks(atbPath)
		candidate := filepath.Join(filepath.Dir(atbPath), binaryName())
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	if path, err := exec.LookPath(binaryName()); err == nil {
		return path, nil
	}

	return "", fmt.Errorf(`agc not found.

Run 'atb agc install' to download it automatically (Linux, macOS, Windows x64).

Or install manually:
  Pre-built: https://github.com/%s/releases/tag/%s
  Conda:     conda install -c bioconda agc`, sources.AGCRepo, sources.AGCVersion)
}
