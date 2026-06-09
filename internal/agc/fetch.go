package agc

import (
	"io"
	"os"
	"path/filepath"

	"github.com/allthebacteria/atb-cli/internal/download"
	"github.com/allthebacteria/atb-cli/internal/sources"
)

// FetchSpec configures a FetchGenomes run.
type FetchSpec struct {
	Combine    bool      // one combined stream instead of per-sample files
	OutputDir  string    // per-sample output directory (ignored when Combine)
	Combined   io.Writer // combined output target (used when Combine)
	ArchiveDir string    // where .agc archives are cached
	BaseURL    string    // archive base URL; "" uses sources.ArchiveURL
	Parallel   int       // parallel archive downloads
	Force      bool      // re-download archives even if cached
	Options    Options   // agc getset flags
}

// FetchResult summarises a FetchGenomes run.
type FetchResult struct {
	Completed int
	Failed    int
	Errors    []FetchError
}

// FetchError records a per-accession failure.
type FetchError struct {
	Accession string
	Error     string
}

// ArchiveDir returns the directory where .agc archives are cached: override
// when set, otherwise <dataDir>/agc. Mirrors the sketch database layout.
func ArchiveDir(dataDir, override string) string {
	if override != "" {
		return override
	}
	return filepath.Join(dataDir, sources.AGCArchiveSubdir)
}

// archiveURL builds the download URL for an archive. An empty baseURL uses the
// provisional default in sources; otherwise it is "<baseURL><archive>.agc".
func archiveURL(baseURL, archive string) string {
	if baseURL == "" {
		return sources.ArchiveURL(archive)
	}
	return baseURL + archive + ".agc"
}

// downloadArchives fetches each archive cache-first into spec.ArchiveDir and
// returns the local path of every archive that is now present, plus a map of
// archive name -> download error for any that failed. Force removes a cached
// copy first so it is re-downloaded.
func downloadArchives(archives []string, spec FetchSpec) (paths map[string]string, errs map[string]string) {
	urlToArchive := make(map[string]string, len(archives))
	tasks := make([]download.FileTask, 0, len(archives))
	for _, a := range archives {
		u := archiveURL(spec.BaseURL, a)
		urlToArchive[u] = a
		if spec.Force {
			// Best-effort: a missing cache file is the desired end state, so an
			// error here (typically the file is simply absent) is ignored; the
			// download below re-creates it.
			os.Remove(filepath.Join(spec.ArchiveDir, a+".agc"))
		}
		tasks = append(tasks, download.FileTask{URL: u, Filename: a + ".agc"})
	}

	dl := download.New(download.Config{OutputDir: spec.ArchiveDir, Parallel: spec.Parallel})
	res := dl.DownloadAllFiles(tasks)

	errs = make(map[string]string)
	for _, e := range res.Errors {
		if a, ok := urlToArchive[e.URL]; ok {
			errs[a] = e.Error
		}
	}
	paths = make(map[string]string)
	for _, a := range archives {
		if _, bad := errs[a]; !bad {
			paths[a] = filepath.Join(spec.ArchiveDir, a+".agc")
		}
	}
	return paths, errs
}
