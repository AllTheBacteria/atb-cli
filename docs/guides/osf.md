# Browsing OSF files

The AllTheBacteria project hosts ~3,000 files on [OSF](https://osf.io/h7wzy/) across 75+ categories (assemblies, annotations, AMR, MLST, protein structures, and more). Use `atb osf` to browse and download them directly.

## Browse and download

```bash
# List all project categories
atb osf ls

# Find files matching a keyword
atb osf ls AMR
atb osf ls "Protein Structures"

# Regex search across project and filename
atb osf ls --grep "bakta.*batch"

# Sort by size, different output formats
atb osf ls AMR --sort size --format json

# Preview what would be downloaded
atb osf download --dry-run "AMRFinderPlus.*results.*latest"

# Download with MD5 verification
atb osf download --verify "DefenseFinder.*results"

# Download all files in a project
atb osf download --project AllTheBacteria/MLST --all -o ./mlst_data
```

## Extract assembly tarballs

The `.tar.xz` batches contain uncompressed FASTA files; `--extract` unpacks them, `--compress` chooses the on-disk encoding (`none`, `gz` (default), or `xz`), and `--delete-archive` removes each tarball once it has extracted successfully:

```bash
# Download all assembly batches and extract FASTAs as .fa.gz, removing tarballs
atb osf download --project AllTheBacteria/Assembly --all \
    --extract --compress gz --delete-archive -o ./assemblies

# One batch, raw FASTA, keep the tarball
atb osf download "Assembly.*batch.*1\." --extract --compress none
```

The file index is cached locally and refreshed every 7 days. Use `--refresh` to force an update.
