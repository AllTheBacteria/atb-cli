package agc

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/allthebacteria/atb-cli/internal/download"
	"github.com/allthebacteria/atb-cli/internal/sources"
)

// FetchSpec configures a FetchGenomes run.
type FetchSpec struct {
	Combine    bool                  // one combined stream instead of per-sample files
	OutputDir  string                // per-sample output directory (ignored when Combine)
	Combined   io.Writer             // combined output target (used when Combine)
	ArchiveDir string                // where .agc archives are cached
	BaseURL    string                // archive base URL; "" uses sources.ArchiveURL
	Refs       map[string]ArchiveRef // archive name -> OSF {URL, md5}; overrides BaseURL per archive
	Parallel   int                   // parallel archive downloads
	Force      bool                  // re-download archives even if cached
	Options    Options               // agc getset/getcol flags
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

// buildDownloadTasks renders one download task per archive and a reverse
// url->archive map for error attribution. When spec.Refs has an entry for the
// archive, its OSF download URL and md5 are used (the URL is an opaque GUID that
// cannot be constructed from the name, and the md5 enables integrity checking);
// otherwise the URL is built from spec.BaseURL and no md5 is set.
func buildDownloadTasks(archives []string, spec FetchSpec) ([]download.FileTask, map[string]string) {
	urlToArchive := make(map[string]string, len(archives))
	tasks := make([]download.FileTask, 0, len(archives))
	for _, a := range archives {
		u := archiveURL(spec.BaseURL, a)
		md5 := ""
		if ref, ok := spec.Refs[a]; ok && ref.URL != "" {
			u = ref.URL
			md5 = ref.MD5
		}
		urlToArchive[u] = a
		tasks = append(tasks, download.FileTask{URL: u, Filename: a + ".agc", MD5: md5})
	}
	return tasks, urlToArchive
}

// downloadArchives fetches each archive cache-first into spec.ArchiveDir and
// returns the local path of every archive that is now present, plus a map of
// archive name -> download error for any that failed. Force removes a cached
// copy first so it is re-downloaded.
func downloadArchives(archives []string, spec FetchSpec) (paths map[string]string, errs map[string]string) {
	if spec.Force {
		for _, a := range archives {
			// Best-effort: a missing cache file is the desired end state, so an
			// error here (typically the file is simply absent) is ignored; the
			// download below re-creates it.
			os.Remove(filepath.Join(spec.ArchiveDir, a+".agc"))
		}
	}

	tasks, urlToArchive := buildDownloadTasks(archives, spec)

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

// sampleFilename is the per-sample output filename for an accession.
func sampleFilename(accession string, gzip bool) string {
	if gzip {
		return accession + ".fa.gz"
	}
	return accession + ".fa"
}

// FetchGenomes downloads each group's archive cache-first, then extracts the
// requested samples with `agc getset`. With Combine, all samples stream to
// spec.Combined; otherwise each sample is written to its own file in
// spec.OutputDir. Failures are collected per accession (continue-on-error); the
// run still returns a FetchResult so the caller can report and set the exit
// code. A download failure fails every accession in that archive's group.
func FetchGenomes(groups map[string][]string, spec FetchSpec) (FetchResult, error) {
	if _, err := FindBinary(); err != nil {
		return FetchResult{}, err
	}

	archives := make([]string, 0, len(groups))
	for a := range groups {
		archives = append(archives, a)
	}
	sort.Strings(archives) // deterministic processing order

	if !spec.Combine && spec.OutputDir != "" {
		if err := os.MkdirAll(spec.OutputDir, 0o755); err != nil {
			return FetchResult{}, fmt.Errorf("create output dir: %w", err)
		}
	}

	paths, dlErrs := downloadArchives(archives, spec)

	var result FetchResult
	for _, archive := range archives {
		accs := groups[archive]
		if msg, bad := dlErrs[archive]; bad {
			dlErr := fmt.Sprintf("download %s.agc: %s", archive, msg)
			if len(accs) == 0 {
				// Whole-archive group (Mode A): the batch is the unit of work, so
				// attribute the failure to the batch name rather than dropping it.
				result.fail(archive, dlErr)
			} else {
				for _, acc := range accs {
					result.fail(acc, dlErr)
				}
			}
			continue
		}
		switch {
		case len(accs) == 0:
			// No specific accessions requested: extract the entire batch (getcol).
			extractWholeArchive(paths[archive], archive, spec, &result)
		case spec.Combine:
			extractCombined(paths[archive], accs, spec, &result)
		default:
			extractPerSample(paths[archive], accs, spec, &result)
		}
	}
	return result, nil
}

// extractPerSample runs one getset per accession into its own file, isolating
// failures so one bad sample does not affect the others.
func extractPerSample(archive string, accs []string, spec FetchSpec, result *FetchResult) {
	gzip := spec.Options.GzipLevel > 0
	for _, acc := range accs {
		path := filepath.Join(spec.OutputDir, sampleFilename(acc, gzip))
		f, err := os.Create(path)
		if err != nil {
			result.fail(acc, err.Error())
			continue
		}
		err = GetSamples(archive, []string{acc}, f, spec.Options)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(path)
			result.fail(acc, err.Error())
			continue
		}
		result.Completed++
	}
}

// extractCombined writes each requested sample into the combined stream, in
// input order. Each sample is extracted into an in-memory buffer first and only
// copied to spec.Combined on success, so a sample that fails partway through
// never leaves partial or duplicated bytes in the combined output. This mirrors
// extractPerSample's remove-the-partial-file-on-failure guarantee for the
// streaming case, and keeps failures isolated per accession (continue-on-error).
// A direct batch getset into spec.Combined is intentionally NOT used: its output
// cannot be rewound, so a partial failure would corrupt or duplicate the stream.
func extractCombined(archive string, accs []string, spec FetchSpec, result *FetchResult) {
	for _, acc := range accs {
		var buf bytes.Buffer
		if err := GetSamples(archive, []string{acc}, &buf, spec.Options); err != nil {
			result.fail(acc, err.Error())
			continue
		}
		if _, err := io.Copy(spec.Combined, &buf); err != nil {
			result.fail(acc, err.Error())
			continue
		}
		result.Completed++
	}
}

// archiveOutputName is the per-batch output filename for whole-archive mode.
func archiveOutputName(archive string, gzip bool) string {
	if gzip {
		return archive + ".fa.gz"
	}
	return archive + ".fa"
}

// extractWholeArchive extracts an entire batch as FASTA with `agc getcol`, the
// path Mode A (by-species) takes when every genome in the batch is wanted. In
// combine mode it streams getcol straight into spec.Combined: whole-archive
// output cannot be rewound and a decompressed batch can be many gigabytes, so
// buffering to isolate a mid-stream failure is impractical — a failure is
// reported per batch (the unit of work here) and may leave partial bytes in the
// combined stream. In per-sample mode it writes one <archive>.fa[.gz] file per
// batch and removes a partial file on failure. archiveName identifies the batch
// in results (continue-on-error, mirroring the per-accession paths).
func extractWholeArchive(archivePath, archiveName string, spec FetchSpec, result *FetchResult) {
	if spec.Combine {
		if err := GetCollection(archivePath, spec.Combined, spec.Options); err != nil {
			result.fail(archiveName, err.Error())
			return
		}
		result.Completed++
		return
	}

	gzip := spec.Options.GzipLevel > 0
	path := filepath.Join(spec.OutputDir, archiveOutputName(archiveName, gzip))
	f, err := os.Create(path)
	if err != nil {
		result.fail(archiveName, err.Error())
		return
	}
	err = GetCollection(archivePath, f, spec.Options)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(path)
		result.fail(archiveName, err.Error())
		return
	}
	result.Completed++
}

func (r *FetchResult) fail(accession, msg string) {
	r.Failed++
	r.Errors = append(r.Errors, FetchError{Accession: accession, Error: msg})
}
