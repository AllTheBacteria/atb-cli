package agc

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/allthebacteria/atb-cli/internal/sources"
)

// CacheMaxAge is how long a cached archive map is considered fresh.
const CacheMaxAge = 7 * 24 * time.Hour

// ArchiveMap maps an ATB sample accession to the archive name (no ".agc")
// that contains it.
type ArchiveMap map[string]string

// Unresolved records an accession that could not be mapped to an archive.
type Unresolved struct {
	Accession string
	Reason    string
}

// ParseMap reads the whitespace-delimited sample->archive map: column 1 is the
// accession, column 2 is the archive name. Extra columns and blank or
// single-column lines are ignored. Lookup is by exact accession (not the
// prototype's substring grep).
func ParseMap(r io.Reader) (ArchiveMap, error) {
	m := make(ArchiveMap)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		m[fields[0]] = fields[1]
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("parse archive map: %w", err)
	}
	return m, nil
}

// FetchMap returns the parsed archive map, using a cached copy in cacheDir when
// it is younger than CacheMaxAge. force=true always re-downloads. An empty
// mapURL falls back to sources.AGCArchiveMapURL. The body is streamed to a
// temp file and renamed into place so a partial download never poisons the
// cache; the file is large (~90 MB).
func FetchMap(cacheDir, mapURL string, force bool) (ArchiveMap, error) {
	if mapURL == "" {
		mapURL = sources.AGCArchiveMapURL
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}
	cached := filepath.Join(cacheDir, sources.AGCArchiveMapFilename)

	if !force {
		if info, err := os.Stat(cached); err == nil && time.Since(info.ModTime()) < CacheMaxAge {
			f, err := os.Open(cached)
			if err != nil {
				return nil, fmt.Errorf("open cached archive map: %w", err)
			}
			defer f.Close()
			return ParseMap(f)
		}
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(mapURL)
	if err != nil {
		return nil, fmt.Errorf("fetch archive map: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch archive map: HTTP %d (set agc.archive_map_url to override)", resp.StatusCode)
	}

	tmp := cached + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(tmp)
		return nil, fmt.Errorf("write archive map: %w", err)
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return nil, fmt.Errorf("close archive map: %w", err)
	}
	if err := os.Rename(tmp, cached); err != nil {
		os.Remove(tmp)
		return nil, fmt.Errorf("rename archive map: %w", err)
	}

	f, err := os.Open(cached)
	if err != nil {
		return nil, fmt.Errorf("open archive map: %w", err)
	}
	defer f.Close()
	return ParseMap(f)
}
