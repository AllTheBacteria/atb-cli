package osf

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/allthebacteria/atb-cli/internal/sources"
)

// speciesToken separates the species prefix from the batch ordinal in an AGC
// archive filename: "<Species>_global_ordered_<NNNN>.agc".
const speciesToken = "_global_ordered_"

// osfAPIPage is one page of an OSF "files" listing
// (GET /v2/nodes/<id>/files/osfstorage/<folder>/). Only the fields atb needs
// are decoded; "next" is null on the final page (decodes to "").
type osfAPIPage struct {
	Data  []osfAPIItem `json:"data"`
	Links struct {
		Next string `json:"next"`
	} `json:"links"`
}

type osfAPIItem struct {
	Attributes struct {
		Name  string  `json:"name"`
		Kind  string  `json:"kind"` // "file" or "folder"
		Size  float64 `json:"size"`
		Extra struct {
			Hashes struct {
				MD5 string `json:"md5"`
			} `json:"hashes"`
		} `json:"extra"`
	} `json:"attributes"`
	Links struct {
		Download string `json:"download"`
	} `json:"links"`
	Relationships struct {
		Files struct {
			Links struct {
				Related struct {
					Href string `json:"href"`
				} `json:"related"`
			} `json:"links"`
		} `json:"files"`
	} `json:"relationships"`
}

// SpeciesFromArchive derives the species prefix from an AGC archive name by
// splitting on the literal "_global_ordered_" token. This keeps GTDB
// letter-suffixes ("Streptococcus_suis_AA") and the special
// "subthreshold_remainder" batch intact. The ".agc" extension is optional.
func SpeciesFromArchive(name string) string {
	stem := strings.TrimSuffix(name, ".agc")
	if i := strings.Index(stem, speciesToken); i >= 0 {
		return stem[:i]
	}
	return stem
}

// parseAGCNodePage decodes one OSF listing page into index entries (one per
// file, folders skipped) and returns the URL of the next page ("" when last).
// ProjectID is left empty here; the crawler stamps it with the node id.
func parseAGCNodePage(r io.Reader) ([]Entry, string, error) {
	var page osfAPIPage
	if err := json.NewDecoder(r).Decode(&page); err != nil {
		return nil, "", fmt.Errorf("decode OSF page: %w", err)
	}
	var entries []Entry
	for _, it := range page.Data {
		if it.Attributes.Kind != "file" {
			continue
		}
		name := it.Attributes.Name
		entries = append(entries, Entry{
			Project:  SpeciesFromArchive(name),
			Filename: name,
			URL:      it.Links.Download,
			MD5:      it.Attributes.Extra.Hashes.MD5,
			SizeMB:   it.Attributes.Size / 1e6,
		})
	}
	return entries, page.Links.Next, nil
}

// crawlMaxPages caps pagination so a malformed "next" cycle cannot loop
// forever; 767 batches at <=100/page is 8 pages, so this is generous.
const crawlMaxPages = 10000

// CrawlAGCNode walks an OSF folder listing starting at startURL, following the
// "next" link until exhausted, and returns an Index of every .agc file found.
// Each entry's ProjectID is stamped with nodeID. A nil client gets a default.
func CrawlAGCNode(client *http.Client, startURL, nodeID string) (*Index, error) {
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	idx := &Index{}
	url := startURL
	for pages := 0; url != "" && pages < crawlMaxPages; pages++ {
		resp, err := client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("crawl OSF node: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("crawl OSF node: HTTP %d", resp.StatusCode)
		}
		entries, next, err := parseAGCNodePage(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		for i := range entries {
			entries[i].ProjectID = nodeID
		}
		idx.Entries = append(idx.Entries, entries...)
		url = next
	}
	return idx, nil
}

// WriteAGCIndexTSV writes the index as a 6-column TSV in the same layout as the
// master OSF index, so ParseIndex round-trips it.
func WriteAGCIndexTSV(idx *Index, w io.Writer) error {
	bw := bufio.NewWriter(w)
	if _, err := fmt.Fprintln(bw, "project\tproject_id\tfilename\turl\tmd5\tsize_mb"); err != nil {
		return err
	}
	for _, e := range idx.Entries {
		size := strconv.FormatFloat(e.SizeMB, 'f', 6, 64)
		if _, err := fmt.Fprintf(bw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			e.Project, e.ProjectID, e.Filename, e.URL, e.MD5, size); err != nil {
			return err
		}
	}
	return bw.Flush()
}

// findFolderURL fetches an OSF node's root listing and returns the contents
// listing URL (the "related" link) of the named subfolder. OSF does not let you
// address a folder by name directly, so the root must be listed to discover the
// opaque per-folder endpoint.
func findFolderURL(client *http.Client, rootURL, folderName string) (string, error) {
	resp, err := client.Get(rootURL)
	if err != nil {
		return "", fmt.Errorf("list OSF node root: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("list OSF node root: HTTP %d", resp.StatusCode)
	}
	var page osfAPIPage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return "", fmt.Errorf("decode OSF node root: %w", err)
	}
	for _, it := range page.Data {
		if it.Attributes.Kind == "folder" && it.Attributes.Name == folderName {
			href := it.Relationships.Files.Links.Related.Href
			if href == "" {
				return "", fmt.Errorf("folder %q has no contents link", folderName)
			}
			return href, nil
		}
	}
	return "", fmt.Errorf("folder %q not found on OSF node", folderName)
}

// CrawlAGCIndex resolves the agc_batches/ folder from an OSF node's root
// listing (rootURL, build it with sources.OSFNodeFilesURL) and crawls every page
// of that folder into an Index, with no caching side effect. Each entry's
// ProjectID is stamped with nodeID. This is the network half of FetchAGCIndex,
// exposed so `atb agc index` can crawl on demand and write the TSV wherever the
// user wants it.
func CrawlAGCIndex(rootURL, nodeID string) (*Index, error) {
	client := &http.Client{Timeout: 2 * time.Minute}
	folderURL, err := findFolderURL(client, rootURL, sources.AGCBatchesFolder)
	if err != nil {
		return nil, err
	}
	return CrawlAGCNode(client, folderURL, nodeID)
}

// agcCacheSourceSuffix names the sidecar file that records which source produced
// the cached AGC index TSV. The cache filename is a constant, so without this
// marker a cache built from one source would be reused for a different source
// until CacheMaxAge expired — e.g. a new release pointing AGCIndexURL at a
// freshly published TSV, or a switch to a different OSF node, would not reach
// users with a warm cache. Comparing the recorded source against the requested
// one makes a source change invalidate the cache at once.
const agcCacheSourceSuffix = ".source"

// readAGCCacheSource returns the source recorded next to the cached index, or ""
// when no marker exists (a cache written by an older atb, or none at all).
func readAGCCacheSource(cachePath string) string {
	b, err := os.ReadFile(cachePath + agcCacheSourceSuffix)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// writeAGCCacheSource records the source that produced the cached index so a
// later fetch can tell whether reuse is safe. Best-effort: the index itself is
// already cached, and a missing marker only forces a harmless re-fetch.
func writeAGCCacheSource(cachePath, source string) {
	_ = os.WriteFile(cachePath+agcCacheSourceSuffix, []byte(source+"\n"), 0644)
}

// agcCacheFresh reports whether the cached index at cachePath may be reused: it
// must exist, be younger than CacheMaxAge, and carry a source marker equal to
// want. A missing or mismatched marker means the cache came from a different
// source (or an older atb) and must be refetched.
func agcCacheFresh(cachePath, want string) bool {
	info, err := os.Stat(cachePath)
	if err != nil || time.Since(info.ModTime()) >= CacheMaxAge {
		return false
	}
	return readAGCCacheSource(cachePath) == want
}

// FetchAGCIndexFromURL returns the AGC batch index by downloading a pre-built
// TSV from url, mirroring FetchIndex: a cached copy under
// <cacheDir>/atb_agc_files.tsv is reused while younger than CacheMaxAge and
// while its source marker still matches url, otherwise the file is downloaded
// and written atomically (alongside a refreshed marker) before parsing. This is
// the hosted counterpart to FetchAGCIndex's live crawl — once the index has been
// published as a single OSF file (sources.AGCIndexURL) there is no need to walk
// the node's agc_batches/ folder page by page. Set force=true to bypass a fresh
// cache.
func FetchAGCIndexFromURL(cacheDir, url string, force bool) (*Index, error) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}
	cached := filepath.Join(cacheDir, sources.AGCIndexFilename)

	if !force && agcCacheFresh(cached, url) {
		f, err := os.Open(cached)
		if err != nil {
			return nil, fmt.Errorf("open cached AGC index: %w", err)
		}
		defer f.Close()
		return ParseIndex(f)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch AGC index: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch AGC index: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read AGC index body: %w", err)
	}

	tmp := cached + ".tmp"
	if err := os.WriteFile(tmp, body, 0644); err != nil {
		return nil, fmt.Errorf("write AGC index: %w", err)
	}
	if err := os.Rename(tmp, cached); err != nil {
		os.Remove(tmp)
		return nil, fmt.Errorf("rename AGC index: %w", err)
	}
	writeAGCCacheSource(cached, url)

	return ParseIndex(strings.NewReader(string(body)))
}

// FetchAGCIndex returns the AGC batch index for an OSF node, mirroring
// FetchIndex: a cached TSV is reused while younger than CacheMaxAge and while
// its source marker still matches rootURL, otherwise the node's agc_batches/
// folder is crawled and the result written atomically to
// <cacheDir>/atb_agc_files.tsv (alongside a refreshed marker). rootURL is the
// node's osfstorage listing (build it with sources.OSFNodeFilesURL); it is a
// parameter so the crawl is testable against a local server. nodeID is stamped
// onto every entry's ProjectID. Set force=true to bypass a fresh cache.
func FetchAGCIndex(cacheDir, rootURL, nodeID string, force bool) (*Index, error) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}
	cached := filepath.Join(cacheDir, sources.AGCIndexFilename)

	if !force && agcCacheFresh(cached, rootURL) {
		f, err := os.Open(cached)
		if err != nil {
			return nil, fmt.Errorf("open cached AGC index: %w", err)
		}
		defer f.Close()
		return ParseIndex(f)
	}

	idx, err := CrawlAGCIndex(rootURL, nodeID)
	if err != nil {
		return nil, err
	}

	tmp := cached + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return nil, fmt.Errorf("create temp AGC index: %w", err)
	}
	if err := WriteAGCIndexTSV(idx, out); err != nil {
		out.Close()
		os.Remove(tmp)
		return nil, fmt.Errorf("write AGC index: %w", err)
	}
	out.Close()
	if err := os.Rename(tmp, cached); err != nil {
		os.Remove(tmp)
		return nil, fmt.Errorf("rename AGC index: %w", err)
	}
	writeAGCCacheSource(cached, rootURL)

	return idx, nil
}
