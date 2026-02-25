.PHONY: build test lint clean

BINARY := talons
BUILD_DIR := bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X github.com/rufinus/talons-console/internal/version.Version=$(VERSION) -s -w"

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/talons/

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BUILD_DIR)/
