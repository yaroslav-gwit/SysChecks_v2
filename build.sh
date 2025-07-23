#!/usr/bin/env bash

# Get version information from git
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(date -u '+%Y-%m-%d_%H:%M:%S_UTC')

# Build with version information
go build -ldflags="-X 'syschecks/cmd.Version=$VERSION' -X 'syschecks/cmd.GitCommit=$COMMIT' -X 'syschecks/cmd.BuildDate=$DATE'" -o ./bin/syschecks
