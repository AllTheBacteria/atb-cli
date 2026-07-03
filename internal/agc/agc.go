// Package agc reads AGC (Assembled Genomes Compressor) archives by shelling
// out to the upstream `agc` binary. atb fetches and caches that binary next to
// itself; agc is never linked into atb, so atb stays pure Go and statically
// linked. The package is read-only: it lists samples/contigs and extracts
// FASTA, but never creates or modifies archives.
package agc

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

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

// InstallBinary downloads the pinned agc release asset for the host platform,
// extracts the binary, and installs it (chmod 0755) next to the atb binary.
func InstallBinary(progress func(string)) error {
	asset, isZip, err := platformAsset(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	atbPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find atb binary: %w", err)
	}
	atbPath, err = filepath.EvalSymlinks(atbPath)
	if err != nil {
		return fmt.Errorf("resolve atb path: %w", err)
	}
	installDir := filepath.Dir(atbPath)

	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s",
		sources.AGCRepo, sources.AGCVersion, asset)
	if progress != nil {
		progress(fmt.Sprintf("Downloading %s...", asset))
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download agc: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download agc: HTTP %d from %s", resp.StatusCode, url)
	}

	if progress != nil {
		progress("Extracting...")
	}

	var binaryPath string
	if isZip {
		binaryPath, err = extractZip(resp.Body, binaryName())
	} else {
		binaryPath, err = extractTarGz(resp.Body, binaryName())
	}
	if err != nil {
		return fmt.Errorf("extract agc: %w", err)
	}
	defer os.Remove(binaryPath)

	destPath := filepath.Join(installDir, binaryName())
	if err := os.Chmod(binaryPath, 0o755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}
	if err := copyFile(binaryPath, destPath); err != nil {
		return fmt.Errorf("install agc to %s: %w", installDir, err)
	}

	if progress != nil {
		progress(fmt.Sprintf("Installed agc %s to %s", sources.AGCVersion, destPath))
	}
	return nil
}

// extractTarGz reads a gzip-compressed tar from r and writes the member whose
// basename equals want to a temp file, returning that file's path.
func extractTarGz(r io.Reader, want string) (string, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return "", fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("tar: %w", err)
		}
		if filepath.Base(header.Name) == want {
			return writeTemp(tr)
		}
	}
	return "", fmt.Errorf("agc binary %q not found in archive", want)
}

// extractZip reads a zip from r (buffered in memory; the asset is a few MB) and
// writes the member whose basename equals want to a temp file.
func extractZip(r io.Reader, want string) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read zip: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("zip: %w", err)
	}
	for _, f := range zr.File {
		if filepath.Base(f.Name) == want {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()
			return writeTemp(rc)
		}
	}
	return "", fmt.Errorf("agc binary %q not found in archive", want)
}

// writeTemp copies src into a new temp file and returns its path. The caller
// owns the file and must remove it.
func writeTemp(src io.Reader) (string, error) {
	tmp, err := os.CreateTemp("", "agc-*")
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
