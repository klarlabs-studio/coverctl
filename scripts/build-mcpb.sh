#!/usr/bin/env bash
set -euo pipefail

# Build MCPB bundles for all platforms
# MCPB = MCP Bundle format (.mcpb is a zip with manifest.json + binary)

VERSION="${1:-$(git describe --tags --abbrev=0 | sed 's/^v//')}"

rm -rf dist/mcpb
mkdir -p dist/mcpb

build_mcpb() {
  local goos=$1
  local goarch=$2
  local platform=$3
  local ext=""
  [[ "$goos" == "windows" ]] && ext=".exe"

  local bundle_name="coverctl-mcp-${goos}-${goarch}.mcpb"
  local work_dir="dist/mcpb/work-${goos}-${goarch}"

  mkdir -p "$work_dir/server"

  # Build binary
  echo "Building binary for ${goos}/${goarch}..."
  env CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -o "$work_dir/server/coverctl${ext}" .

  # Create manifest.json
  cat > "$work_dir/manifest.json" <<EOF
{
  "manifest_version": "0.3",
  "name": "coverctl",
  "version": "${VERSION}",
  "description": "Go code coverage analysis tool with MCP server for AI-powered coverage workflows",
  "author": {
    "name": "Felix Geelhaar",
    "url": "https://github.com/felixgeelhaar"
  },
  "server": {
    "type": "binary",
    "entry_point": "server/coverctl${ext}",
    "mcp_config": {
      "command": "server/coverctl${ext}",
      "args": ["mcp", "serve"],
      "env": {}
    }
  },
  "compatibility": {
    "platforms": ["${platform}"]
  },
  "repository": "https://github.com/klarlabs-studio/coverctl",
  "homepage": "https://github.com/klarlabs-studio/coverctl"
}
EOF

  # Create .mcpb (zip) bundle
  echo "Creating ${bundle_name}..."
  (cd "$work_dir" && zip -r "../../${bundle_name}" manifest.json server/)

  # Cleanup work dir
  rm -rf "$work_dir"

  echo "Created dist/${bundle_name}"
}

# Build for all platforms
build_mcpb linux amd64 linux
build_mcpb linux arm64 linux
build_mcpb darwin amd64 darwin
build_mcpb darwin arm64 darwin
build_mcpb windows amd64 win32
build_mcpb windows arm64 win32

echo ""
echo "MCPB bundles created in dist/"
ls -la dist/*.mcpb
