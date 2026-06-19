BINARY     := atb
BIN_DIR    := bin
CMD_PATH   := ./cmd/atb
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS    := -ldflags "-X main.version=$(VERSION)"

GO         := go
GOFLAGS    ?=
# Build a statically linked binary so releases run on older glibc systems
# (e.g. RHEL 7/8/9, Debian 11). All dependencies are pure-Go.
export CGO_ENABLED ?= 0

.PHONY: build test lint clean fixtures docs docs-serve docs-build

build:
	mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY) $(CMD_PATH)

test:
	$(GO) test ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BIN_DIR)

fixtures:
	$(GO) run ./internal/testdata/gen/... 2>/dev/null || true

docs:
	$(GO) run ./cmd/gen-docs

docs-serve: docs
	mkdocs serve

docs-build: docs
	mkdocs build --strict
