# Move `atb fetch-genomes` under `agc` as `atb agc download`

- **Date:** 2026-06-28
- **Branch:** `feat/agc-osf-test` (WIP — **do not merge to main**; no push/PR without confirmation)
- **Status:** Design approved → implementation

## Problem

The genome-retrieval surface mixed two vocabularies. `atb download` (released)
fetches genome FASTA from S3; `atb fetch-genomes` (unreleased) fetched genome
FASTA from AGC archives but sat at the top level under a *different* verb, and
the `agc` group's help actively pointed users away to it. A newcomer could not
tell whether AGC retrieval was `atb fetch-genomes` or some `atb agc` subcommand.

## Decision

- AGC genome retrieval **nests under the `agc` group**.
- The verb is **`download`**, matching released `atb download` — both mean "get
  genome FASTA". `fetch` stays reserved for "get the metadata DB".
- Net rename: `atb fetch-genomes` → `atb agc download`. Because the command has
  never shipped in a release, this is a **clean removal** — no deprecation alias.

## Surface

| | Before | After |
|---|--------|-------|
| AGC genome FASTA | `atb fetch-genomes` (+ `genomes` alias) | `atb agc download` |
| `agc` subcommands | `install index ls info get` | `download install index ls info get` |
| `atb download` (S3 FASTA, released) | unchanged | unchanged |
| `atb fetch` (metadata DB, released) | unchanged | unchanged |

All three input modes carry over unchanged: positional accessions, `--from`
file/stdin, and `--species` whole-batch.

## Code changes

| File | Change |
|------|--------|
| `internal/cli/fetch_genomes_cmd.go` → **`agc_download_cmd.go`** | `newFetchGenomesCmd()` → `newAGCDownloadCmd()`; `Use: "download [accession...]"`; drop `Aliases`; rewrite `Short`/`Long`/`Example` to `atb agc download` |
| `internal/cli/fetch_genomes_helpers.go` → **`agc_download_helpers.go`** | rename file only (shared helpers; no logic change) |
| `internal/cli/agc_cmd.go` | `cmd.AddCommand(newAGCDownloadCmd())`; rewrite the group `Long` so `download` is the headline and `ls`/`info`/`get` are framed as local-file tools |
| `internal/cli/root.go` | remove **both** top-level `newFetchGenomesCmd()` registrations (the global `RootCmd` and the `NewRootCmd` test root) |
| `internal/cli/agc_index_cmd.go` | help text `atb fetch-genomes --species` → `atb agc download --species` |
| `internal/cli/fetch_genomes_cmd_test.go` → **`agc_download_cmd_test.go`** | tests drive `runCmd("agc", "download", …)`; rename test funcs to `TestAGCDownload*`; add a test that top-level `fetch-genomes` is now unknown |
| `internal/cli/fetch_genomes_helpers_test.go` → **`agc_download_helpers_test.go`** | rename file only |

`agc.go` (`FindBinary`/`InstallBinary`) and the `internal/agc` + `internal/osf`
packages are untouched — only the CLI wiring moves.

## Flag decision: keep `--refresh`

`download --force` means "overwrite output + skip the disk-space check";
`agc download --refresh` means "invalidate the cached index TSV and `.agc`
archives and re-fetch them". Different behaviors — renaming would mislead. Keep
`--refresh` as-is.

## TDD

1. **RED** — repoint the help test to `runCmd("agc", "download", "--help")`
   (fails: `download` is not yet a subcommand of `agc`); add a test asserting
   top-level `atb fetch-genomes` now errors as an unknown command (fails: it
   still exists). Watch both fail for the right reason.
2. **GREEN** — move/rename the command, register it under `agc`, drop the two
   root registrations. Repoint the remaining dry-run/species/missing-binary
   tests to `agc download`.
3. **Gate** — `go build ./... && go vet ./... && go test ./...` plus a manual
   `git diff` review, substituting for the degraded GitNexus MCP
   (`gitnexus_impact`/`detect_changes` unavailable this session).

## Docs

`atb fetch-genomes` → `atb agc download` across `docs/guides/agc.md`,
`docs/design/agc-osf-test-summary.md`, `agc-osf-test-implementation.md`,
`agc-osf-migration.md`, `README.md`, and the `agc` group/index help text.

## Out of scope (YAGNI)

Renaming the released `download`/`fetch`; a `fetch-genomes` back-compat alias;
merging the S3 and AGC backends; touching `osf download`'s `--refresh`; a full
`FOR-DEVELOPERS.md` (deferred while this branch is WIP).
