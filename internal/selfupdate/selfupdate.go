package selfupdate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	repo          = "allthebacteria/atb-cli"
	checkInterval = 24 * time.Hour
	stateFileName = "update-state.json"
)

type Release struct {
	TagName string  `json:"tag_name"`
	HTMLURL string  `json:"html_url"`
	Body    string  `json:"body"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type UpdateState struct {
	LastChecked   time.Time `json:"last_checked"`
	LatestVersion string    `json:"latest_version"`
	ReleaseNotes  string    `json:"release_notes,omitempty"`
	ReleaseURL    string    `json:"release_url,omitempty"`
	NotifiedUser  bool      `json:"notified_user"`
}

func statePath() string {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "atb", stateFileName)
	}
	switch runtime.GOOS {
	case "windows":
		if appdata := os.Getenv("LOCALAPPDATA"); appdata != "" {
			return filepath.Join(appdata, "atb", stateFileName)
		}
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "atb", stateFileName)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "atb", stateFileName)
}

func loadState() UpdateState {
	data, err := os.ReadFile(statePath())
	if err != nil {
		return UpdateState{}
	}
	var state UpdateState
	json.Unmarshal(data, &state)
	return state
}

func saveState(state UpdateState) {
	path := statePath()
	os.MkdirAll(filepath.Dir(path), 0o755)
	data, _ := json.Marshal(state)
	os.WriteFile(path, data, 0o644)
}

func FetchLatestRelease() (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parse release: %w", err)
	}
	return &release, nil
}

// CompareVersions returns true if remote is newer than current.
// Both should be semver strings like "v0.1.0" or "0.1.0".
func CompareVersions(current, remote string) bool {
	current = strings.TrimPrefix(current, "v")
	remote = strings.TrimPrefix(remote, "v")

	if current == "dev" || current == "" {
		return false // dev builds don't auto-update
	}

	cur := parseSemver(current)
	rem := parseSemver(remote)

	if cur == nil || rem == nil {
		return remote != current && remote > current
	}

	for i := 0; i < 3; i++ {
		if rem[i] > cur[i] {
			return true
		}
		if rem[i] < cur[i] {
			return false
		}
	}
	return false
}

func parseSemver(v string) []int {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return nil
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n := 0
		for _, c := range p {
			if c < '0' || c > '9' {
				return nil
			}
			n = n*10 + int(c-'0')
		}
		nums[i] = n
	}
	return nums
}

func assetName() string {
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("atb-cli_*_%s_%s.%s", runtime.GOOS, runtime.GOARCH, ext)
}

// FindAsset finds the matching release asset for the current platform.
func FindAsset(release *Release) *Asset {
	target := fmt.Sprintf("_%s_%s.", runtime.GOOS, runtime.GOARCH)
	for i := range release.Assets {
		if strings.Contains(release.Assets[i].Name, target) {
			return &release.Assets[i]
		}
	}
	return nil
}

// Apply downloads the release asset and replaces the current binary.
func Apply(asset *Asset) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(asset.BrowserDownloadURL)
	if err != nil {
		return fmt.Errorf("download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "atb-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("write update: %w", err)
	}
	tmp.Close()

	// Extract binary from archive
	binaryPath, err := extractBinary(tmpPath, asset.Name)
	if err != nil {
		return err
	}
	defer os.Remove(binaryPath)

	// Replace current binary
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find current binary: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolve binary path: %w", err)
	}

	if err := os.Chmod(binaryPath, 0o755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	// Check if we can write to the target directory
	if needsElevation(execPath) {
		return installWithSudo(binaryPath, execPath)
	}

	return installDirect(binaryPath, execPath)
}

func needsElevation(path string) bool {
	if runtime.GOOS == "windows" {
		return false // Windows handles permissions differently
	}
	dir := filepath.Dir(path)
	testFile := filepath.Join(dir, ".atb-write-test")
	f, err := os.Create(testFile)
	if err != nil {
		return true
	}
	f.Close()
	os.Remove(testFile)
	return false
}

func installDirect(binaryPath, execPath string) error {
	backupPath := execPath + ".old"
	os.Remove(backupPath)
	if err := os.Rename(execPath, backupPath); err != nil {
		return fmt.Errorf("backup current binary: %w", err)
	}
	if err := copyFile(binaryPath, execPath); err != nil {
		os.Rename(backupPath, execPath)
		return fmt.Errorf("install new binary: %w", err)
	}
	os.Remove(backupPath)
	return nil
}

func installWithSudo(binaryPath, execPath string) error {
	fmt.Fprintf(os.Stderr, "  Installing to %s requires elevated permissions.\n", filepath.Dir(execPath))
	// Remove first to avoid "Text file busy" when the binary is running
	rmCmd := exec.Command("sudo", "rm", "-f", execPath)
	rmCmd.Stdin = os.Stdin
	rmCmd.Stdout = os.Stderr
	rmCmd.Stderr = os.Stderr
	_ = rmCmd.Run() // ignore error if file doesn't exist

	cpCmd := exec.Command("sudo", "cp", binaryPath, execPath)
	cpCmd.Stdin = os.Stdin
	cpCmd.Stdout = os.Stderr
	cpCmd.Stderr = os.Stderr
	if err := cpCmd.Run(); err != nil {
		return fmt.Errorf("sudo install failed: %w\n\nYou can also install manually:\n  sudo rm -f %s && sudo cp %s %s", err, execPath, binaryPath, execPath)
	}
	return nil
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
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// CheckInBackground performs a non-blocking update check and prints a notice
// if a newer version is available. Call from root command PersistentPreRun.
//
// Returns a function that waits (up to a short timeout) for the background
// check to finish. The caller should defer this in main() so the goroutine
// has time to save its state before the process exits.
func CheckInBackground(currentVersion string, w io.Writer) func() {
	noop := func() {}

	state := loadState()
	if time.Since(state.LastChecked) < checkInterval {
		// Already checked recently — show notice if a newer version was found.
		if state.LatestVersion != "" && CompareVersions(currentVersion, state.LatestVersion) {
			printUpdateNotice(w, currentVersion, state)
		}
		return noop
	}

	// Check in a background goroutine so the actual command is not blocked.
	done := make(chan struct{})
	go func() {
		defer close(done)
		release, err := FetchLatestRelease()
		if err != nil {
			return // silently fail
		}
		state.LastChecked = time.Now()
		state.LatestVersion = release.TagName
		state.ReleaseNotes = release.Body
		state.ReleaseURL = release.HTMLURL
		saveState(state)

		if CompareVersions(currentVersion, release.TagName) {
			printUpdateNotice(w, currentVersion, state)
		}
	}()

	// Return a wait function: gives the goroutine up to 2 seconds to finish
	// so the state file is written before the process exits.
	return func() {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}
}

func printUpdateNotice(w io.Writer, currentVersion string, state UpdateState) {
	fmt.Fprintf(w, "\n  A new version of atb is available: %s (current: %s)\n", state.LatestVersion, currentVersion)
	if state.ReleaseNotes != "" {
		fmt.Fprintf(w, "\n  What's new:\n")
		for _, line := range strings.Split(strings.TrimSpace(state.ReleaseNotes), "\n") {
			fmt.Fprintf(w, "    %s\n", line)
		}
	}
	if state.ReleaseURL != "" {
		fmt.Fprintf(w, "\n  Release: %s\n", state.ReleaseURL)
	}
	fmt.Fprintf(w, "\n  Run 'atb update' to upgrade.\n\n")
}
