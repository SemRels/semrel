BIN_DIR      := bin
BINARY       := $(BIN_DIR)/go-semrel
VERSION      ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS      := -X github.com/GoSemantics/go-semrel/internal/cli.version=$(VERSION)
GOFLAGS      := -trimpath

.PHONY: all build test lint reuse clean tidy

all: build

build:
	@mkdir -p $(BIN_DIR)
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/go-semrel

test:
	go test ./... -race -count=1 -coverprofile=coverage.txt -covermode=atomic

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
