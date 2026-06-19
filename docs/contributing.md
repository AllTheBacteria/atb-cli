# Contributing

Contributions are welcome — bug reports, feature requests, and pull requests. This page covers building, testing, and working on the documentation.

## Building

```bash
# Build for current platform
make build

# Run tests
make test

# Cross-compile for all supported platforms
GOOS=linux   GOARCH=amd64 go build -o bin/atb-linux-amd64   ./cmd/atb
GOOS=linux   GOARCH=arm64 go build -o bin/atb-linux-arm64   ./cmd/atb
GOOS=darwin  GOARCH=amd64 go build -o bin/atb-darwin-amd64  ./cmd/atb
GOOS=darwin  GOARCH=arm64 go build -o bin/atb-darwin-arm64  ./cmd/atb
GOOS=windows GOARCH=amd64 go build -o bin/atb-windows-amd64.exe ./cmd/atb
GOOS=windows GOARCH=arm64 go build -o bin/atb-windows-arm64.exe ./cmd/atb
```

Requires Go 1.23+. Pure Go, no CGO - cross-compilation works out of the box.

## Editing these docs

The CLI reference under `docs/reference/cli/` is generated automatically — do not edit those files by hand.

| Task | Command |
|------|---------|
| Regenerate CLI reference | `make docs` |
| Preview the site locally | `make docs-serve` |
| Build and validate the site | `make docs-build` |

`make docs` runs `go run ./cmd/gen-docs` to regenerate the Markdown from the live command set. `make docs-serve` regenerates the reference and then starts `mkdocs serve` for a live preview at `http://localhost:8000`. `make docs-build` regenerates the reference and then runs `mkdocs build --strict`, mirroring the CI gate locally.

In CI, the reference is regenerated and the build fails if the committed `docs/reference/cli/` is out of date — so always run `make docs` before pushing documentation changes.

## Publishing on ReadTheDocs

A maintainer performs a one-time import of the repository at [readthedocs.org](https://readthedocs.org). ReadTheDocs auto-detects the committed `.readthedocs.yaml` at the repo root, which pins the build environment (`ubuntu-24.04`, Python 3.12) and points at `mkdocs.yml`. After that initial import, every push to the main branch triggers an automatic build and publish — no further manual steps are needed.
