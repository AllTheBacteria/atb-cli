package agc

import (
	"archive/zip"
	"bufio"
	"bytes"
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

// openMap returns a reader over the accession->archive map stored at path,
// transparently decompressing when the file is a ZIP. The upstream artifact is a
// ZIP of a single text file; a plain-text file is read as-is so a future
// uncompressed publish still works. Every ZIP record begins "PK" (a populated
// archive with PK\x03\x04, a valid-but-empty one with PK\x05\x06), so any
// PK-prefixed file is routed to the zip reader - which surfaces an empty archive
// as an error instead of a silently empty map. The map's first column is always
// an accession, never "PK", so plain text is unambiguous. The caller must Close
// the returned reader, which owns any underlying zip handle.
func openMap(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	magic := make([]byte, 2)
	n, _ := io.ReadFull(f, magic)
	f.Close()
	if n == 2 && bytes.Equal(magic, []byte("PK")) {
		return openZipMap(path)
	}
	return os.Open(path)
}

// openZipMap opens the single entry inside the ZIP at path.
func openZipMap(path string) (io.ReadCloser, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open zip map %s: %w", path, err)
	}
	if len(zr.File) == 0 {
		zr.Close()
		return nil, fmt.Errorf("zip map %s is empty", path)
	}
	rc, err := zr.File[0].Open()
	if err != nil {
		zr.Close()
		return nil, fmt.Errorf("open zip entry in %s: %w", path, err)
	}
	return &zipEntryReadCloser{rc: rc, zr: zr}, nil
}

// zipEntryReadCloser ties a zip entry reader to its parent archive so both close
// together.
type zipEntryReadCloser struct {
	rc io.ReadCloser
	zr *zip.ReadCloser
}

func (z *zipEntryReadCloser) Read(p []byte) (int, error) { return z.rc.Read(p) }

func (z *zipEntryReadCloser) Close() error {
	err := z.rc.Close()
	if cerr := z.zr.Close(); err == nil {
		err = cerr
	}
	return err
}

// FetchMap returns the parsed archive map, using a cached copy in cacheDir when
// it is younger than CacheMaxAge. force=true always re-downloads. An empty
// mapURL falls back to sources.AGCArchiveMapURL. The body is streamed to a
// temp file and renamed into place so a partial download never poisons the
// cache; the file is large. The cached artifact is stored exactly as downloaded
// (a ZIP) and openMap decompresses it on read.
func FetchMap(cacheDir, mapURL string, force bool) (ArchiveMap, error) {
	if mapURL == "" {
		mapURL = sources.AGCArchiveMapURL
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}
	cached := filepath.Join(cacheDir, sources.AGCArchiveMapFilename)

	fresh := false
	if !force {
		if info, err := os.Stat(cached); err == nil && time.Since(info.ModTime()) < CacheMaxAge {
			fresh = true
		}
	}
	if !fresh {
		if err := downloadMap(cached, mapURL); err != nil {
			return nil, err
		}
	}

	rc, err := openMap(cached)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return ParseMap(rc)
}

// downloadMap streams mapURL to cached atomically (a partial download never
// poisons the cache; the file is large). The artifact is stored exactly as
// downloaded - openMap handles decompression on read.
func downloadMap(cached, mapURL string) error {
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(mapURL)
	if err != nil {
		return fmt.Errorf("fetch archive map: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch archive map: HTTP %d (set agc.archive_map_url to override)", resp.StatusCode)
	}

	tmp := cached + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(tmp)
		return fmt.Errorf("write archive map: %w", err)
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("close archive map: %w", err)
	}
	if err := os.Rename(tmp, cached); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename archive map: %w", err)
	}
	return nil
}

// ResolveArchives maps each accession to its archive using the companion map
// (cached under <dataDir>/agc), grouping accessions by archive and preserving
// input order within each group. Accessions absent from the map are returned in
// unresolved rather than failing the whole call. refresh=true re-downloads the
// map. The SQLite index fast-path is intentionally deferred; this signature is
// forward-compatible with adding it later.
func ResolveArchives(dataDir, mapURL string, accessions []string, refresh bool) (groups map[string][]string, unresolved []Unresolved, err error) {
	cacheDir := filepath.Join(dataDir, sources.AGCArchiveSubdir)
	m, err := FetchMap(cacheDir, mapURL, refresh)
	if err != nil {
		return nil, nil, err
	}
	groups = make(map[string][]string)
	for _, acc := range accessions {
		archive, ok := m[acc]
		if !ok {
			unresolved = append(unresolved, Unresolved{Accession: acc, Reason: "not found in archive map"})
			continue
		}
		groups[archive] = append(groups[archive], acc)
	}
	return groups, unresolved, nil
}
