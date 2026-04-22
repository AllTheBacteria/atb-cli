package osf

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/allthebacteria/atb-cli/internal/sources"
)

const CacheMaxAge = 7 * 24 * time.Hour

type Entry struct {
	Project   string
	ProjectID string
	Filename  string
	URL       string
	MD5       string
	SizeMB    float64
}

type ProjectSummary struct {
	Project   string
	FileCount int
	TotalMB   float64
}

type Index struct {
	Entries []Entry
}

// FetchIndex returns a parsed index, using a cached copy if fresh enough.
// Set force=true to always re-download.
func FetchIndex(cacheDir string, force bool) (*Index, error) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}

	cached := filepath.Join(cacheDir, sources.IndexFilename)

	if !force {
		if info, err := os.Stat(cached); err == nil {
			if time.Since(info.ModTime()) < CacheMaxAge {
				f, err := os.Open(cached)
				if err != nil {
					return nil, fmt.Errorf("open cached index: %w", err)
				}
				defer f.Close()
				return ParseIndex(f)
			}
		}
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(sources.IndexURL)
	if err != nil {
		return nil, fmt.Errorf("fetch index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch index: HTTP %d", resp.StatusCode)
	}

	tmp := cached + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		out.Close()
		os.Remove(tmp)
		return nil, fmt.Errorf("read index body: %w", err)
	}

	if _, err := out.Write(body); err != nil {
		out.Close()
		os.Remove(tmp)
		return nil, fmt.Errorf("write index: %w", err)
	}
	out.Close()

	if err := os.Rename(tmp, cached); err != nil {
		os.Remove(tmp)
		return nil, fmt.Errorf("rename index: %w", err)
	}

	return ParseIndex(strings.NewReader(string(body)))
}

// ParseIndex reads the TSV index from r.
// Expected columns: project, project_id, filename, url, md5, size(MB)
func ParseIndex(r io.Reader) (*Index, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	// Skip header
	if !scanner.Scan() {
		return nil, fmt.Errorf("empty index file")
	}

	var entries []Entry
	lineNum := 1
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) < 6 {
			continue
		}

		sizeMB, err := strconv.ParseFloat(fields[5], 64)
		if err != nil {
			sizeMB = 0
		}

		entries = append(entries, Entry{
			Project:   fields[0],
			ProjectID: fields[1],
			Filename:  fields[2],
			URL:       fields[3],
			MD5:       fields[4],
			SizeMB:    sizeMB,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan index: %w", err)
	}

	return &Index{Entries: entries}, nil
}

// Projects returns a summary of each unique project with file count and total size.
func (idx *Index) Projects() []ProjectSummary {
	m := make(map[string]*ProjectSummary)
	for _, e := range idx.Entries {
		s, ok := m[e.Project]
		if !ok {
			s = &ProjectSummary{Project: e.Project}
			m[e.Project] = s
		}
		s.FileCount++
		s.TotalMB += e.SizeMB
	}

	out := make([]ProjectSummary, 0, len(m))
	for _, s := range m {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Project < out[j].Project
	})
	return out
}

// Filter returns entries matching the given criteria.
// If project is non-empty, only entries whose Project starts with that prefix are included.
// If pattern is non-empty, it is compiled as a regex and matched against "project/filename".
func (idx *Index) Filter(project, pattern string) ([]Entry, error) {
	var re *regexp.Regexp
	if pattern != "" {
		var err error
		re, err = regexp.Compile("(?i)" + pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
		}
	}

	var out []Entry
	for _, e := range idx.Entries {
		if project != "" && !strings.HasPrefix(e.Project, project) {
			continue
		}
		if re != nil {
			combined := e.Project + "/" + e.Filename
			if !re.MatchString(combined) {
				continue
			}
		}
		out = append(out, e)
	}
	return out, nil
}

// MatchProject returns entries whose project matches a case-insensitive substring.
func (idx *Index) MatchProject(query string) []Entry {
	lower := strings.ToLower(query)
	var out []Entry
	for _, e := range idx.Entries {
		if strings.Contains(strings.ToLower(e.Project), lower) {
			out = append(out, e)
		}
	}
	return out
}
