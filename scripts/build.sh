#!/usr/bin/env bash
set -euo pipefail

VERSION="${VERSION:-dev}"
OUTPUT_DIR="${OUTPUT_DIR:-dist}"
mkdir -p "$OUTPUT_DIR"

build() {
  local goos="$1"
  local goarch="$2"
  local ext=""
  if [[ "$goos" == "windows" ]]; then
    ext=".exe"
  fi
  local out="$OUTPUT_DIR/modelpilot-${goos}-${goarch}${ext}"
  echo "Building $out"
  GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$out" ./cmd/modelpilot
}

build windows amd64
build linux amd64
build darwin amd64
build darwin arm64

echo "Build artifacts written to $OUTPUT_DIR"
