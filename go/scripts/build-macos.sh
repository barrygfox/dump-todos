#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
DIST_DIR="$GO_DIR/dist"
APP_NAME="dump-todos"

mkdir -p "$DIST_DIR"

build_target() {
  local arch="$1"
  local output="$DIST_DIR/${APP_NAME}-darwin-${arch}"

  echo "Building ${output}"
  GOOS=darwin GOARCH="$arch" CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$output" ./cmd/dump-todos
}

cd "$GO_DIR"

build_target arm64
build_target amd64

echo "Builds written to $DIST_DIR"