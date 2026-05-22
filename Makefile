BINARY := devgrep
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X github.com/devgrep/devgrep/cmd.Version=$(VERSION) -X github.com/devgrep/devgrep/cmd.Commit=$(COMMIT) -X github.com/devgrep/devgrep/cmd.Date=$(DATE)

.PHONY: build test lint bench release clean

build:
	go build -buildvcs=false -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) .

test:
	go test -race -coverprofile=coverage.out ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		go vet ./...; \
	fi

bench:
	go test -bench=. -benchmem ./...

release:
	goreleaser release --clean

clean:
	rm -rf bin dist coverage.out
