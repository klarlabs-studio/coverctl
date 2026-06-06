#!/usr/bin/env bash
set -euo pipefail

rm -rf dist
mkdir -p dist

# Version info from git
VERSION="${GITHUB_REF_NAME#v}"
if [ -z "$VERSION" ] || [ "$VERSION" = "$GITHUB_REF_NAME" ]; then
  VERSION=$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "dev")
fi
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS="-s -w \
  -X go.klarlabs.de/coverctl/internal/cli.Version=${VERSION} \
  -X go.klarlabs.de/coverctl/internal/cli.Commit=${COMMIT} \
  -X go.klarlabs.de/coverctl/internal/cli.Date=${DATE}"

echo "Building version: ${VERSION} (commit: ${COMMIT})"

build() {
  local goos=$1
  local goarch=$2
  local name=$3
  local output=dist/$name
  env CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -ldflags "$LDFLAGS" -o "$output" .
  case "$goos" in
    linux)
      tar -C dist -czf "$output.tar.gz" "$name"
      rm "$output"
      ;;
    darwin)
      tar -C dist -czf "$output.tar.gz" "$name"
      rm "$output"
      ;;
    windows)
      zip -j dist/"$name.zip" "$output"
      rm "$output"
      ;;
  esac
}

# Linux builds
build linux amd64 coverctl-linux-amd64
build linux arm64 coverctl-linux-arm64

# macOS builds
build darwin amd64 coverctl-darwin-amd64
build darwin arm64 coverctl-darwin-arm64

# Windows builds
build windows amd64 coverctl-windows-amd64.exe
build windows arm64 coverctl-windows-arm64.exe

echo "Build complete. Artifacts in dist/"
