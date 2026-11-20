# Release 1 parquet converter

- **Date:** 2026-11-20
- **Code:** `internal/convert` (package), `cmd/convert-r1` (command)
- **Status:** Standalone tool. It is not part of the `atb` command.

## Purpose

AllTheBacteria release 1 publishes its metadata as gzipped TSV files
(`atb.metadata.r1.*.tsv.gz`). The `atb` CLI reads parquet files. `convert-r1`
converts the release 1 TSVs into the five parquet files that `atb index` and
`atb query` read, with no change to the CLI:

| Output | Read by |
|---|---|
| `assembly.parquet` | `pq.AssemblyRow`, index build |
| `assembly_stats.parquet` | `pq.AssemblyStatsRow`, index build |
| `checkm2.parquet` | `pq.CheckM2Row`, index build |
| `sylph.parquet` | `pq.SylphRow` |
| `run.parquet` | `pq.RunRow` |

The converter copies every row and every column. It does not filter, rename or
recompute anything.

## Inputs and outputs

| Table | Input | Output |
|---|---|---|
| assembly | `atb.metadata.r1.release_1.tsv.gz` | `assembly.parquet` |
| assembly_stats | `atb.metadata.r1.assembly_stats.tsv.gz` | `assembly_stats.parquet` |
| checkm2 | `atb.metadata.r1.checkm2.tsv.gz` | `checkm2.parquet` |
| sylph | `atb.metadata.r1.sylph.tsv.gz` | `sylph.parquet` |
| run | `atb.metadata.r1.run.tsv.gz` | `run.parquet` |

`assembly.parquet` is written from the `release_1` export, not from the
`assembly` export. The `release_1` export holds only release 1 samples (3,092,278
rows). The `assembly` export has 3,720,978 rows. It also holds 575,574
`not_processed` samples, 30,937 `reject` samples and 22,189 samples from
released datasets that release 1 does not include.

The TSV reader splits each line on `\t` and does no quote handling. A field
may contain `"` characters and they are kept as they are.

Each output is first written to `<name>.parquet.tmp`. The converter then
reopens it, checks that the footer row count equals the number of input rows
and that the footer columns match the table spec, and renames it to
`<name>.parquet`. On any error the `.tmp` file is removed and an existing
output is left unchanged.

After the tables are converted, the command writes `MD5SUMS` in the
`md5sum` format (`<md5>  <file>`), one line for each table output present in
the output directory.

File layout:

- Every column is OPTIONAL.
- Column order is the TSV header order.
- Compression is ZSTD at the default level.
- Each row group holds 100,000 rows. The last row group holds the rest.
- String columns use PLAIN encoding. The parquet-go default for byte arrays
  is DELTA_LENGTH_BYTE_ARRAY. With that default, `run.parquet` was 238 MB and
  its `fastq_md5` column 202 MB. With PLAIN they are 170 MB and 136 MB.

## Column types

Each column has one of three kinds. The kinds are declared per column in
`internal/convert/tables.go`.

| Kind | Parquet type |
|---|---|
| String | BYTE_ARRAY, logical type STRING (UTF8) |
| Int64 | INT64 |
| Float64 | DOUBLE |

Parsing is strict. An Int64 value must parse with `strconv.ParseInt`. A
Float64 value may contain only `0-9 + - . e E` and must parse with
`strconv.ParseFloat`, so `NaN`, `Inf`, hexadecimal and `_` forms are rejected.
A String value must be valid UTF-8.

The converter stops at the first problem. Each error names the file, the line
number and, for a value error, the column and the value:

```text
atb.metadata.r1.checkm2.tsv.gz line 1: header column 3 is "Contam", want "Contamination"
atb.metadata.r1.run.tsv.gz line 5021: row has 17 fields, want 18
atb.metadata.r1.assembly_stats.tsv.gz line 88: column N50: invalid int64 value "1.5"
atb.metadata.r1.release_1.tsv.gz line 42: column in_release_1: value "0", want "1"
```

The header must match the spec exactly, in name and order. Every row must have
as many fields as the header.

`in_release_1` in the assembly table must be `1` on every row. A row with any
other value stops the conversion.

## Null rules

| Kind | Null when the field is | Kept as a value |
|---|---|---|
| Int64, Float64 | empty or `NA` | every other value |
| String | empty | everything else, including `NA` and `None` |
| String with `NAIsNull` | empty or `NA` | everything else |

Only two columns set `NAIsNull`: `Completeness_Model_Used` and
`Additional_Notes` in checkm2.

Examples of strings kept as they are: `osf_tarball_url` in assembly holds
`NA` for 344,425 samples and `comments` holds `None` for 3,035,095 samples.
The old parquet files hold the same strings.

Two checkm2 values differ from the old files because of these rules. The CLI
reads neither column:

- `Completeness_Model_Used`: the old file holds the string `NA` in 297 rows.
  The new file holds null there.
- `Additional_Notes`: the TSV holds `NA` for older samples, which becomes
  null, and the string `None` for 349,541 newer samples, which is kept.

## Validation results

### Tests and checks

- `go build ./...`, `go vet`, `gofmt -l` and `go mod tidy -diff` are clean for
  the converter package and command.
- `go test ./internal/convert/... ./cmd/convert-r1/...` passes.

### Conversion

`/usr/bin/time -v convert-r1 --in-dir r1_tsv --out-dir r1_parquet`:

```text
assembly        input rows   3092278  output rows   3092278  28.543s
assembly_stats  input rows   3127929  output rows   3127929  12.509s
checkm2         input rows   3116707  output rows   3116707  12.469s
sylph           input rows   4305650  output rows   4305650  25.008s
run             input rows   3986805  output rows   3986805  26.643s
```

Wall time 1:46, user time 128 s, peak RSS 624 MiB. Peak RSS for one table:

| Command | Peak RSS |
|---|---:|
| `--tables run` (no coverage report) | 188 MiB |
| `--tables assembly` (builds the set of 3.09 million assembly samples) | 497 MiB |
| all five tables | 624 MiB |

| File | TSV.gz (MB) | Old parquet (MB) | New parquet (MB) |
|---|---:|---:|---:|
| assembly | 139.4 | 95.2 | 114.5 |
| assembly_stats | 96.7 | 111.2 | 87.1 |
| checkm2 | 83.4 | 74.1 | 70.7 |
| sylph | 137.4 | 109.1 | 134.1 |
| run | 189.1 | 148.1 | 170.0 |

### Coverage report

```text
assembly: 3092278 rows, 3092278 distinct samples
           table     rows  samples  rows not in assembly  assembly samples missing
  assembly_stats  3127929  3127929                 35651                         0
         checkm2  3116707  3116707                 24429                         0
           sylph  4305650  3113332                 86743                      7795
             run  3986805  3718471                801531                      1896
```

- Every assembly sample has assembly_stats and checkm2 rows.
- 7,795 assembly samples have no sylph rows and 1,896 have no run rows.
- 1,027 sylph rows and 3,709 run rows have a comma-separated
  `sample_accession`, for example `SAMN18762348,SAMN18762349`. These rows
  count as "not in assembly". All 3,709 such run rows have `pass = 0`.

### Schema compared with the old parquet files

Checked with pyarrow against `/data/atb/metadata/parquet`:

- No column was removed. Every shared column has the same name, the same type
  (`string` in the new files, `large_string` in the old ones) and the same
  order.
- Every column is nullable. String columns read as `string`, not `binary`.
- New columns in assembly: `duplicate_seqkit_sum_status`, `in_release_1`,
  `agc_filename`, `agc_url`, `agc_md5`.
- New columns in run: `in_ena_20260317`, `failure_reasons`,
  `latest_ena_source`, `latest_ena_observed_date`, `first_seen_date`,
  `last_seen_date`.
- assembly_stats, checkm2 and sylph have the same columns as before.
- The old files also use 100,000-row row groups.

### Values compared with the old parquet files

Rows were joined on `sample_accession` (assembly, assembly_stats, checkm2), on
`sample_accession, run_accession, Genome_file` (sylph) and on `run_accession`
(run). Two nulls count as a match.

| Table | Shared rows | Columns | Match |
|---|---:|---|---:|
| assembly | 2,747,853 | `run_accession`, `assembly_accession`, `sylph_species`, `dataset`, `scientific_name`, `aws_url`, `asm_fasta_on_osf`, `osf_tarball_url` | 100% |
| assembly | 2,747,853 | `hq_filter` | 99.14% |
| assembly_stats | 2,778,266 | `total_length`, `number`, `mean_length`, `longest`, `shortest`, `N50`, `N90` | 100% |
| checkm2 | 2,767,044 | `Completeness_General`, `Contamination`, `Completeness_Specific`, `Genome_Size`, `GC_Content` | 100% |
| sylph | 3,815,195 | `Species`, `Adjusted_ANI`, `Taxonomic_abundance`, `Sequence_abundance`, `Median_cov` | 100% |
| run | 3,530,512 | `sample_accession` | 99.998% |
| run | 3,530,512 | `pass` | 99.93% |

The differences are in the release 1 data, not in the conversion:

- `hq_filter` differs on 23,540 samples. Release 1 adds the flags `NO_RUNS`,
  `RMMS` and `RUN_REMOVED`, for example `META_FAIL` became
  `META_FAIL,NO_RUNS`. 2,640 samples went from `PASS` to not `PASS` and 169
  went the other way.
- `run.sample_accession` differs on 58 runs. Each new value joins the old
  sample with another one, for example `SAMN18762349` became
  `SAMN18762348,SAMN18762349`.
- `run.pass` went from 1 to 0 on 2,564 runs and never from 0 to 1.

Every old assembly_stats, checkm2, sylph and run row is present in the new
files. The old `assembly.parquet` has 479,812 samples that are not in the new
one: 432,478 `not_processed`, 30,937 `reject` and 16,397 from released datasets
that release 1 does not include. The new file has 344,425 samples that are not
in the old one, all from `Incr_release.202603`.

### Reading with internal/parquet

`internal/parquet.ReadAll` reads every file into its row type
(`AssemblyRow`, `AssemblyStatsRow`, `CheckM2Row`, `SylphRow`, `RunRow`) and
returns 3,092,278, 3,127,929, 3,116,707, 4,305,650 and 3,986,805 rows.

### End to end with atb index

The five files were copied to a scratch data directory and indexed with
`atb --config <scratch>/config.toml --data-dir <scratch> index --force`, with
`XDG_STATE_HOME`, `XDG_DATA_HOME`, `XDG_CONFIG_HOME` and `XDG_CACHE_HOME`
pointed at scratch directories. The build took 4 min 27 s and
indexed 3,092,278 samples. `atb query --dataset <name>` returns:

| Dataset | Samples |
|---|---:|
| 661k | 657,476 |
| r0.2 | 1,265,759 |
| Incr_release.202408 | 504,805 |
| Incr_release.202505 | 319,813 |
| Incr_release.202603 | 344,425 |

## How to run

```bash
go build -o convert-r1 ./cmd/convert-r1

# All five tables
./convert-r1 --in-dir /path/to/r1_tsv --out-dir /path/to/r1_parquet

# Some tables. assembly is always converted first when it is listed.
./convert-r1 --in-dir /path/to/r1_tsv --out-dir /path/to/r1_parquet --tables run,assembly
```

| Flag | Default | Meaning |
|---|---|---|
| `--in-dir` | `.` | Directory holding the `atb.metadata.r1.*.tsv.gz` files |
| `--out-dir` | none, required | Directory for the parquet files and `MD5SUMS`; created if missing |
| `--tables` | all five | Comma-separated table names |

The coverage report is printed only when `assembly` is among the converted
tables. To check the outputs, run `md5sum -c MD5SUMS` in the output directory.

To use the files with the CLI, copy them into a data directory and rebuild the
index:

```bash
atb --data-dir /path/to/data index --force
```
