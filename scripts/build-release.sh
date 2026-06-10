#!/usr/bin/env bash

set -e

VERSION=$(git describe --tags --exact-match)
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

mkdir -p dist

echo "Building ${VERSION}"

GOOS=linux GOARCH=amd64 \
go build \
-ldflags="-s -w \
-X 'github.com/aasixh/devgrep/cmd.Version=${VERSION}' \
-X 'github.com/aasixh/devgrep/cmd.Commit=${COMMIT}' \
-X 'github.com/aasixh/devgrep/cmd.Date=${DATE}'" \
-o dist/devgrep-linux-amd64 .

GOOS=windows GOARCH=amd64 \
go build \
-ldflags="-s -w \
-X 'github.com/aasixh/devgrep/cmd.Version=${VERSION}' \
-X 'github.com/aasixh/devgrep/cmd.Commit=${COMMIT}' \
-X 'github.com/aasixh/devgrep/cmd.Date=${DATE}'" \
-o dist/devgrep-windows-amd64.exe .

GOOS=darwin GOARCH=amd64 \
go build \
-ldflags="-s -w \
-X 'github.com/aasixh/devgrep/cmd.Version=${VERSION}' \
-X 'github.com/aasixh/devgrep/cmd.Commit=${COMMIT}' \
-X 'github.com/aasixh/devgrep/cmd.Date=${DATE}'" \
-o dist/devgrep-darwin-amd64 .

GOOS=darwin GOARCH=arm64 \
go build \
-ldflags="-s -w \
-X 'github.com/aasixh/devgrep/cmd.Version=${VERSION}' \
-X 'github.com/aasixh/devgrep/cmd.Commit=${COMMIT}' \
-X 'github.com/aasixh/devgrep/cmd.Date=${DATE}'" \
-o dist/devgrep-darwin-arm64 .

echo "Done."
