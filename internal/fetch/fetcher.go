package fetch

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/allthebacteria/atb-cli/internal/sources"
)

// CoreTables returns the names of the core parquet tables.
func CoreTables() []string {
	out := make([]string, len(sources.CoreTables))
	copy(out, sources.CoreTables)
	return out
}

// AllTables returns the names of all available parquet tables.
func AllTables() []string {
	out := make([]string, 0, len(sources.TableURLs))
	for name := range sources.TableURLs {
		out = append(out, name)
	}
	return out
}

// URLForTable returns the download URL for a named table, and whether it exists.
func URLForTable(name string) (string, bool) {
	u, ok := sources.TableURLs[name]
	return u, ok
}

// Config holds configuration for a Fetcher.
type Config struct {
	BaseURL  string
	DataDir  string
	Parallel int
}

// Fetcher downloads parquet tables from OSF into a local data directory.
type Fetcher struct {
	cfg    Config
	client *http.Client
}

// New creates a Fetcher with the given configuration.
func New(cfg Config) *Fetcher {
	return &Fetcher{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Minute},
	}
}

// FetchTable downloads a single parquet table by name, using an atomic
// rename (.tmp -> final) to avoid partial writes. If force is false and
// the final file already exists, it is skipped.
func (f *Fetcher) FetchTable(name, url string, force bool) error {
	if err := os.MkdirAll(f.cfg.DataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	final := filepath.Join(f.cfg.DataDir, name)
	if !force {
		if _, err := os.Stat(final); err == nil {
			return nil // already exists
		}
	}

	tmp := final + ".tmp"

	resp, err := f.client.Get(url)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetching %s: HTTP %d", name, resp.StatusCode)
	}

	out, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("creating temp file for %s: %w", name, err)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(tmp)
		return fmt.Errorf("writing %s: %w", name, err)
	}
	out.Close()

	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("renaming %s: %w", name, err)
	}

	return nil
}
