package agc

import (
	"sort"
	"strings"

	"github.com/allthebacteria/atb-cli/internal/osf"
)

// ArchiveRef is a single AGC batch resolved from the OSF index: its archive
// name (without ".agc"), the opaque OSF download URL, the md5 for integrity, and
// the size in MB. Carrying the URL in the data is the key adaptation for OSF —
// archive download links are per-file GUIDs that cannot be built from the name,
// so the engine must be handed a real URL rather than constructing one.
type ArchiveRef struct {
	Name    string
	URL     string
	MD5     string
	SizeMB  float64
	Node    string // OSF node id (osf.Entry.ProjectID); "" for non-collection sources
	Species string // index species (osf.Entry.Project); "" when unknown
}

// archiveStem strips the ".agc" extension from a batch filename, yielding the
// key space the fetch engine groups by.
func archiveStem(filename string) string {
	return strings.TrimSuffix(filename, ".agc")
}

// refFromEntry adapts an OSF index entry into an ArchiveRef.
func refFromEntry(e osf.Entry) ArchiveRef {
	return ArchiveRef{
		Name:    archiveStem(e.Filename),
		URL:     e.URL,
		MD5:     e.MD5,
		SizeMB:  e.SizeMB,
		Node:    e.ProjectID,
		Species: e.Project,
	}
}

// RefsFromIndex keys every batch in the index by its archive stem, the same key
// space the fetch engine uses for its work groups. This is the R2 resolution
// table: archive name -> {URL, md5}.
func RefsFromIndex(idx *osf.Index) map[string]ArchiveRef {
	refs := make(map[string]ArchiveRef, len(idx.Entries))
	for _, e := range idx.Entries {
		ref := refFromEntry(e)
		refs[ref.Name] = ref
	}
	return refs
}

// RefsForGroups splits a ResolveArchives result against the batch index. For
// every archive in groups it looks up the index refs (byName, from
// RefsFromIndex): archives found are returned in refs (which feeds
// FetchSpec.Refs exactly as species-mode does); archives absent from the index -
// a batch named by the map but not yet crawlable, e.g. a collection node still
// uploading - are returned, sorted, in missing, so the caller can report them as
// "not yet available" distinctly from an unknown accession. refs is always
// non-nil.
func RefsForGroups(groups map[string][]string, byName map[string]ArchiveRef) (refs map[string]ArchiveRef, missing []string) {
	refs = make(map[string]ArchiveRef, len(groups))
	for archive := range groups {
		if ref, ok := byName[archive]; ok {
			refs[archive] = ref
		} else {
			missing = append(missing, archive)
		}
	}
	sort.Strings(missing)
	return refs, missing
}

// WholeArchiveGroups turns a by-species selection into the two maps the fetch
// engine needs for Mode A. groups keys every batch name to an empty accession
// slice — the whole-archive sentinel that routes FetchGenomes through `agc
// getcol` (extract the entire batch) instead of per-accession `getset`. byName
// is the R2 table (batch name -> {URL, md5}) handed to FetchSpec.Refs so each
// archive downloads from its opaque OSF GUID with a free md5 check. Both maps are
// always non-nil; keying by Name collapses any accidental duplicate refs.
func WholeArchiveGroups(refs []ArchiveRef) (groups map[string][]string, byName map[string]ArchiveRef) {
	groups = make(map[string][]string, len(refs))
	byName = make(map[string]ArchiveRef, len(refs))
	for _, r := range refs {
		groups[r.Name] = nil
		byName[r.Name] = r
	}
	return groups, byName
}

// normalizeSpecies canonicalises a species query for comparison: surrounding
// whitespace trimmed, internal spaces folded to underscores (so "Acinetobacter
// baylyi" matches the index's "Acinetobacter_baylyi"), and lower-cased.
func normalizeSpecies(s string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), " ", "_"))
}

// SelectBySpecies returns every batch whose species (the index Project column)
// matches the query exactly after normalisation. Matching is case-insensitive
// and treats spaces and underscores as equivalent. GTDB letter-suffixed species
// ("Streptococcus_suis_AA") and the "subthreshold_remainder" batch require their
// full name; a bare prefix does not match. The result may be empty, which the
// caller reports as "no batches for that species".
func SelectBySpecies(idx *osf.Index, species string) []ArchiveRef {
	want := normalizeSpecies(species)
	var out []ArchiveRef
	for _, e := range idx.Entries {
		if normalizeSpecies(e.Project) == want {
			out = append(out, refFromEntry(e))
		}
	}
	return out
}
