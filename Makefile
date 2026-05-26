BIN_DIR      := bin
BINARY       := $(BIN_DIR)/semrel
VERSION      ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS      := -X github.com/GoSemantics/semrel/internal/cli.version=$(VERSION)
GOFLAGS      := -trimpath

.PHONY: all build test coverage lint reuse clean tidy dry-run smoke-test

all: build

build:
	@mkdir -p $(BIN_DIR)
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/semrel

test:
	go test ./... -race -count=1 -coverprofile=coverage.txt -covermode=atomic

# Show coverage summary after running tests
coverage: test
	@echo ""
	@go tool cover -func=coverage.txt | grep -E "^total:" || go tool cover -func=coverage.txt | tail -1
	@go tool cover -html=coverage.txt -o coverage.html
	@echo "Full report: coverage.html"

lint:
	golangci-lint run ./...

# REUSE / SPDX compliance check (requires: pip install reuse)
reuse:
	reuse lint

tidy:
	go mod tidy

clean:
	rm -rf $(BIN_DIR) coverage.txt coverage.html

# Run a local dry-run release against the current repo
dry-run: build
	$(BINARY) release --dry-run

# Smoke-test all plugin binaries found in sibling directories.
# Clone plugin repos next to this repo first, e.g.:
#   git clone https://github.com/SemRels/hook-slack ../hook-slack
# Then run:
#   make smoke-test
smoke-test:
	@bash scripts/smoke-test-plugins.sh
