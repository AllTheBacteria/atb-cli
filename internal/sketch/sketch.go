package sketch

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/allthebacteria/atb-cli/internal/sources"
)

const BinaryName = "sketchlib"

type DatabaseInfo struct {
	Samples    int
	KmerSizes  []int
	SketchSize int
}

// FindBinary locates the sketchlib binary. Search order:
// 1. Same directory as the running atb binary
// 2. System PATH
// If not found, returns an error with install instructions.
func FindBinary() (string, error) {
	// Check next to the atb binary first
	if atbPath, err := os.Executable(); err == nil {
		atbPath, _ = filepath.EvalSymlinks(atbPath)
		candidate := filepath.Join(filepath.Dir(atbPath), BinaryName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// Check PATH
	if path, err := exec.LookPath(BinaryName); err == nil {
		return path, nil
	}

	return "", fmt.Errorf(`sketchlib not found.

Run 'atb sketch install' to download it automatically (Linux/macOS).

Or install manually:
  Pre-built: https://github.com/%s/releases/latest
  Cargo:     cargo install sketchlib
  Conda:     conda install -c bioconda sketchlib

Note: sketch features are not available on Windows.`, sources.SketchlibRepo)
}

// InstallBinary downloads the sketchlib binary from GitHub releases and places
// it alongside the atb binary. Returns the installed path.
func InstallBinary(progress func(string)) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("sketchlib pre-built binaries are not available for Windows.\nInstall from source: cargo install sketchlib")
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

	// Determine platform asset name
	var platform string
	switch runtime.GOOS {
	case "linux":
		platform = "ubuntu-latest"
	case "darwin":
		platform = "macOS-latest"
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	assetName := fmt.Sprintf("sketchlib-%s-%s-stable.tar.gz", sources.SketchlibVersion, platform)
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", sources.SketchlibRepo, sources.SketchlibVersion, assetName)

	if progress != nil {
		progress(fmt.Sprintf("Downloading %s...", assetName))
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download sketchlib: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download sketchlib: HTTP %d from %s", resp.StatusCode, url)
	}

	// Extract sketchlib binary from tar.gz
	if progress != nil {
		progress("Extracting...")
	}

	binaryPath, err := extractSketchlib(resp.Body)
	if err != nil {
		return fmt.Errorf("extract sketchlib: %w", err)
	}
	defer os.Remove(binaryPath)

	// Install to the same directory as atb
	destPath := filepath.Join(installDir, BinaryName)
	if err := os.Chmod(binaryPath, 0o755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	if err := copyFile(binaryPath, destPath); err != nil {
		return fmt.Errorf("install sketchlib to %s: %w", installDir, err)
	}

	if progress != nil {
		progress(fmt.Sprintf("Installed sketchlib %s to %s", sources.SketchlibVersion, destPath))
	}

	return nil
}

func extractSketchlib(r io.Reader) (string, error) {
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

		name := filepath.Base(header.Name)
		if name == BinaryName {
			tmp, err := os.CreateTemp("", "sketchlib-*")
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(tmp, tr); err != nil {
				tmp.Close()
				os.Remove(tmp.Name())
				return "", err
			}
			tmp.Close()
			return tmp.Name(), nil
		}
	}
	return "", fmt.Errorf("sketchlib binary not found in archive")
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

func Info(skmPath string) (*DatabaseInfo, error) {
	bin, err := FindBinary()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, "info", skmPath)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("sketchlib info: %w", err)
	}
	return ParseInfo(strings.NewReader(string(out)))
}

func ParseInfo(r io.Reader) (*DatabaseInfo, error) {
	info := &DatabaseInfo{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]
		switch key {
		case "n_samples":
			info.Samples, _ = strconv.Atoi(val)
		case "sketch_size":
			info.SketchSize, _ = strconv.Atoi(val)
		case "kmers":
			val = strings.Trim(val, "[] ")
			for _, s := range strings.Split(val, ",") {
				s = strings.TrimSpace(s)
				if n, err := strconv.Atoi(s); err == nil {
					info.KmerSizes = append(info.KmerSizes, n)
				}
			}
		}
	}
	return info, scanner.Err()
}

type Match struct {
	RefName   string
	QueryName string
	ANI       float64
}

func ParseDistOutput(r io.Reader) ([]Match, error) {
	var matches []Match
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "sketchlib done") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		ani, err := strconv.ParseFloat(fields[2], 64)
		if err != nil {
			continue
		}
		matches = append(matches, Match{
			RefName:   fields[0],
			QueryName: fields[1],
			ANI:       ani,
		})
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].ANI > matches[j].ANI
	})
	return matches, scanner.Err()
}

// resolveThreads returns the thread count to use: if n <= 0, uses NumCPU - 1
// to leave one core free for the system.
func resolveThreads(n int) int {
	if n <= 0 {
		cpus := runtime.NumCPU() - 1
		if cpus < 1 {
			cpus = 1
		}
		return cpus
	}
	return n
}

func SketchQuery(inputs []string, kmerSizes []int, threads int) (tmpDir, prefix string, err error) {
	bin, err := FindBinary()
	if err != nil {
		return "", "", err
	}
	tmpDir, err = os.MkdirTemp("", "atb-sketch-*")
	if err != nil {
		return "", "", fmt.Errorf("create temp dir: %w", err)
	}
	prefix = filepath.Join(tmpDir, "query")
	kStrs := make([]string, len(kmerSizes))
	for i, k := range kmerSizes {
		kStrs[i] = strconv.Itoa(k)
	}
	t := resolveThreads(threads)
	args := []string{"sketch", "-o", prefix, "--k-vals", strings.Join(kStrs, ","), "--threads", strconv.Itoa(t)}
	args = append(args, inputs...)
	cmd := exec.Command(bin, args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		return "", "", fmt.Errorf("sketchlib sketch: %w", err)
	}
	return tmpDir, prefix, nil
}

func QueryDist(refPrefix, queryPrefix string, kmer, threads, topN int) ([]Match, error) {
	bin, err := FindBinary()
	if err != nil {
		return nil, err
	}
	t := resolveThreads(threads)
	args := []string{"dist", refPrefix, queryPrefix, "-k", strconv.Itoa(kmer), "--ani", "--threads", strconv.Itoa(t)}
	cmd := exec.Command(bin, args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("sketchlib dist: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("sketchlib dist: %w", err)
	}
	matches, err := ParseDistOutput(strings.NewReader(string(out)))
	if err != nil {
		return nil, err
	}
	if topN > 0 && len(matches) > topN {
		matches = matches[:topN]
	}
	return matches, nil
}
