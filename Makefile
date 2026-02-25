BINARY     := talons
CMD        := ./cmd/talons
MODULE     := github.com/rufinus/talons-console
VERSION    ?= dev
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -s -w \
              -X $(MODULE)/internal/version.Version=$(VERSION) \
              -X $(MODULE)/internal/version.Commit=$(COMMIT) \
              -X $(MODULE)/internal/version.BuildDate=$(BUILD_DATE)

GOPATH     := $(shell go env GOPATH)
LINT       := $(GOPATH)/bin/golangci-lint

.PHONY: all build build-all lint test test-race clean install help

## all: Build the binary for the current platform
all: build

## build: Build the binary for the current platform
build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY) $(CMD)

## build-all: Cross-compile for all platforms
build-all:
	@mkdir -p dist
	GOOS=linux  GOARCH=amd64  go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64    $(CMD)
	GOOS=linux  GOARCH=arm64  go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64    $(CMD)
	GOOS=darwin GOARCH=amd64  go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64   $(CMD)
	GOOS=darwin GOARCH=arm64  go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64   $(CMD)
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe $(CMD)

## lint: Run golangci-lint (zero-warning policy)
lint:
	$(LINT) run ./...

## test: Run unit tests
test:
	go test ./...

## test-race: Run unit tests with race detector
test-race:
	go test -race -coverprofile=coverage.out ./...

## coverage: Show test coverage
coverage: test-race
	go tool cover -html=coverage.out

## install: Install the binary to $GOPATH/bin
install:
	go install -ldflags="$(LDFLAGS)" $(CMD)

## clean: Remove build artifacts
clean:
	rm -f $(BINARY)
	rm -f coverage.out
	rm -rf dist/

## help: Show this help
help:
	@echo "Available targets:"
	@sed -n 's/^## //p' Makefile | column -t -s ':' | sed 's/^/  /'
