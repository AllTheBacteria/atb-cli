// Package sources is the single source of truth for all external data URLs
// that atb downloads. Every URL the tool fetches from the internet is defined
// here, grouped by feature.
//
// OSF project: https://osf.io/h7wzy/files/osfstorage
// AllTheBacteria: https://allthebacteria.org
//
// To update a URL: change it here and it propagates everywhere.
// To audit what atb downloads: read this file.
package sources

// ---------------------------------------------------------------------------
// OSF file index
// ---------------------------------------------------------------------------

// IndexURL is the master TSV index listing ~3,000 files across the entire
// AllTheBacteria project on OSF, with download URLs and MD5 checksums.
// Used by: atb osf ls, atb osf download
// Source: https://osf.io/r6gcp (all_atb_files.tsv)
const IndexURL = "https://osf.io/download/r6gcp/"

// IndexFilename is the local cache filename for the OSF file index.
const IndexFilename = "all_atb_files.tsv"

// ---------------------------------------------------------------------------
// Parquet metadata tables
// ---------------------------------------------------------------------------
// Source: https://osf.io/h7wzy/files/osfstorage
// Path:   Aggregated/Latest_2025-05/atb.metadata.202505.parquet/

// TableURLs maps parquet table filenames to their OSF download URLs.
var TableURLs = map[string]string{
	// Core tables (downloaded by default with `atb fetch`)
	"assembly.parquet":       "https://osf.io/download/4ku2n/",                    // sample-to-assembly mapping, species calls, quality flags
	"assembly_stats.parquet": "https://osf.io/download/69c51e86801fecc5d6146396/", // N50, total length, contig counts
	"checkm2.parquet":        "https://osf.io/download/69c51e93cba7111bb21d27f2/", // completeness, contamination, genome size
	"sylph.parquet":          "https://osf.io/download/69c51f90cba7111bb21d2905/", // species-level profiling results
	"run.parquet":            "https://osf.io/download/69c51f68376eb79a651d2d85/", // run accession mapping
	"mlst.parquet":           "https://osf.io/download/69c66d33fa3d973d94254f46/", // multi-locus sequence typing
	"amrfinderplus.parquet":  "https://osf.io/download/69f1e5debb4f674d5fd949ad/", // AMR, stress, virulence genes (AMRFinderPlus v4.2.5, ~58.5M rows)

	// ENA metadata tables (downloaded with `atb fetch --all`)
	"ena_20250506.parquet":    "https://osf.io/download/69c51f3ab4f99c692d54cf73/", // ENA metadata snapshot 2025-05-06
	"ena_20240801.parquet":    "https://osf.io/download/69c51f002e72f67915145d0e/", // ENA metadata snapshot 2024-08-01
	"ena_20240625.parquet":    "https://osf.io/download/69c51ec99ce80b96ac54cd08/", // ENA metadata snapshot 2024-06-25
	"ena_202505_used.parquet": "https://osf.io/download/69c51f475eedad376954ce7b/", // ENA records used in 2025-05 release
	"ena_661k.parquet":        "https://osf.io/download/69c51f57376eb79a651d2d83/", // ENA 661k subset
}

// CoreTables lists the tables downloaded by default with `atb fetch`.
var CoreTables = []string{
	"assembly.parquet",
	"assembly_stats.parquet",
	"checkm2.parquet",
	"sylph.parquet",
	"run.parquet",
	"mlst.parquet",
	"amrfinderplus.parquet",
}

// ---------------------------------------------------------------------------
// Sketch database (sketchlib)
// ---------------------------------------------------------------------------
// Source: https://osf.io/h7wzy/files/osfstorage
// Path:   Aggregated/atb_sketchlib.aggregated.202408.*

// SketchSkmURL is the sketch metadata file (.skm) for the aggregated ATB
// sketch database covering all genomes up to Aug 2024 (~122 MB).
// Used by: atb sketch fetch
const SketchSkmURL = "https://osf.io/download/nwfkc/"

// SketchSkdURL is the sketch data file (.skd) for the aggregated ATB sketch
// database (~4.1 GB).
// Used by: atb sketch fetch
const SketchSkdURL = "https://osf.io/download/92qmr/"

// SketchSkmFilename and SketchSkdFilename are local filenames for the sketch
// database files, stored in <data-dir>/sketch/.
const (
	SketchSkmFilename = "atb_sketchlib.skm"
	SketchSkdFilename = "atb_sketchlib.skd"
	SketchSubdir      = "sketch"
)

// ---------------------------------------------------------------------------
// sketchlib binary
// ---------------------------------------------------------------------------

// SketchlibVersion is the pinned version of the sketchlib binary.
const SketchlibVersion = "v0.2.4"

// SketchlibRepo is the GitHub repository for sketchlib releases.
const SketchlibRepo = "bacpop/sketchlib.rust"

// ---------------------------------------------------------------------------
// agc binary (Assembled Genomes Compressor)
// ---------------------------------------------------------------------------

// AGCVersion is the pinned release tag of the agc binary. agc 3.x reads v1/v2/v3
// archives, so this single pin covers every .agc file in existence today.
const AGCVersion = "v3.2.3"

// AGCRepo is the GitHub repository for agc releases.
const AGCRepo = "refresh-bio/agc"

// ---------------------------------------------------------------------------
// AGC genome archives
// ---------------------------------------------------------------------------
// AllTheBacteria distributes its genome batches as AGC archives hosted on OSF.
// The accession-to-batch map and the local cache layout are isolated here so a
// host change is a one-line edit. Override at runtime via config keys
// agc.archive_map_url / agc.archive_base_url.

// AGCArchiveMapURL is the accession->batch list for the balanced v202505
// collection, published on OSF as a gzipped two-column text file
// (assemblies_filelist.txt.gz: accession, batch name without .agc). atb caches
// the gzip as downloaded and decompresses on read.
const AGCArchiveMapURL = "https://osf.io/download/gtqrx/"

// AGCArchiveMapFilename is the local cache filename for the archive map. It keeps
// the .gz extension because the artifact is cached exactly as downloaded.
const AGCArchiveMapFilename = "assemblies_filelist.txt.gz"

// AGCArchiveSubdir is the <data-dir> subdirectory holding cached archives and
// the archive map. Mirrors SketchSubdir.
const AGCArchiveSubdir = "agc"

// ---------------------------------------------------------------------------
// AGC index over OSF
// ---------------------------------------------------------------------------
// The AGC batches are not registered in the master all_atb_files.tsv, so atb
// builds a SEPARATE index TSV: each batch carries its own opaque
// /download/<guid>/ URL, md5, and size. atb reads the published combined index
// when it is available (AGCIndexURL) and crawls the collection nodes to rebuild
// it otherwise. The collection layout is defined below.

// OSFAPIBase is the root of the OSF REST API (v2).
const OSFAPIBase = "https://api.osf.io/v2"

// AGCIndexFilename is the local cache filename for the AGC index TSV, stored in
// <data-dir>/agc/ alongside the cached archives.
const AGCIndexFilename = "atb_agc_files.tsv"

// AGCIndexURL is the OSF /download/ URL of the published combined AGC index TSV
// (crawl + metadata join, produced by `atb agc index`). When set, the runtime
// downloads this single published TSV; an empty value falls back to crawling the
// collection nodes and joining the metadata on demand.
const AGCIndexURL = "https://osf.io/download/6a719381a2e9e3d202b91f7d/"

// OSFNodeFilesURL returns the osfstorage root listing URL for an OSF node, the
// entry point for crawling its folders.
func OSFNodeFilesURL(nodeID string) string {
	return OSFAPIBase + "/nodes/" + nodeID + "/files/osfstorage/"
}

// ---------------------------------------------------------------------------
// AGC balanced collection over OSF (v202505)
// ---------------------------------------------------------------------------
// The balanced ATB v202505 AGC collection is hosted across three public OSF
// nodes, each under an agc_batches/ folder of numbered .agc batches. These
// batches are NOT registered in the master index; atb builds a combined index
// by crawling the OSF API across all three nodes and joining the batch
// metadata. Kept isolated so a future combined hosted TSV can be dropped in
// without touching production paths.

// AGCNode identifies one OSF node in the collection.
type AGCNode struct {
	ID string
}

// AGCCollectionNodes are the OSF nodes that together host the balanced v202505
// collection; each has an agc_batches/ folder of numbered .agc batches.
var AGCCollectionNodes = []AGCNode{
	{ID: "4jq8u"},
	{ID: "jmeqg"},
	{ID: "kzcnr"},
}

// AGCArchivesFolder is the folder holding .agc batches on the collection nodes.
const AGCArchivesFolder = "agc_batches"

// AGCBatchMetadataURL is the OSF /download/ URL of the batch metadata TSV
// (8y9r2, batches_202505_metadata.tsv.gz): a gzipped TSV with batch_name and
// old_name columns. old_name carries the species; the numbered batch filename
// does not, so `atb agc index` joins this on batch_name to fill the species
// column of the combined index.
const AGCBatchMetadataURL = "https://osf.io/download/8y9r2/"

// ---------------------------------------------------------------------------
// Genome assemblies
// ---------------------------------------------------------------------------

// AssemblyBaseURL is the S3 bucket hosting individual genome FASTA files.
// File pattern: {sample_accession}.fa.gz
// Used by: atb download, atb sketch query --download
const AssemblyBaseURL = "https://allthebacteria-assemblies.s3.eu-west-2.amazonaws.com/"
