#!/bin/sh

VERSION=$(git describe --tags 2>/dev/null || echo "unknown")
BUILD=$(git rev-parse HEAD 2>/dev/null)

go mod download

go build \
    -a -ldflags "-s -w -extldflags '-static' -X main.Version=$VERSION -X main.Build=$BUILD" \
    -o /backuper ./cmd/backuper/main.go
