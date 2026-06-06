#!/usr/bin/env bash
set -euo pipefail

# Update server.json with correct version and SHA256 hashes for MCPB bundles
# Usage: ./scripts/update-server-json.sh [VERSION]

VERSION="${1:-$(git describe --tags --abbrev=0 | sed 's/^v//')}"

if [[ ! -d "dist" ]] || [[ -z "$(ls -A dist/*.mcpb 2>/dev/null)" ]]; then
    echo "Error: No MCPB files found in dist/. Run ./scripts/build-mcpb.sh first."
    exit 1
fi

echo "Updating server.json for version ${VERSION}..."

# Compute SHA256 hashes
DARWIN_ARM64_SHA=$(shasum -a 256 dist/coverctl-mcp-darwin-arm64.mcpb | cut -d' ' -f1)
DARWIN_AMD64_SHA=$(shasum -a 256 dist/coverctl-mcp-darwin-amd64.mcpb | cut -d' ' -f1)
LINUX_AMD64_SHA=$(shasum -a 256 dist/coverctl-mcp-linux-amd64.mcpb | cut -d' ' -f1)
LINUX_ARM64_SHA=$(shasum -a 256 dist/coverctl-mcp-linux-arm64.mcpb | cut -d' ' -f1)
WINDOWS_AMD64_SHA=$(shasum -a 256 dist/coverctl-mcp-windows-amd64.mcpb | cut -d' ' -f1)
WINDOWS_ARM64_SHA=$(shasum -a 256 dist/coverctl-mcp-windows-arm64.mcpb | cut -d' ' -f1)

cat > server.json <<EOF
{
  "\$schema": "https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json",
  "name": "io.github.klarlabs-studio/coverctl",
  "description": "Go code coverage analysis tool with MCP server for AI-powered coverage workflows",
  "repository": {
    "url": "https://github.com/klarlabs-studio/coverctl",
    "source": "github"
  },
  "version": "${VERSION}",
  "packages": [
    {
      "registryType": "mcpb",
      "identifier": "https://github.com/klarlabs-studio/coverctl/releases/download/v${VERSION}/coverctl-mcp-darwin-arm64.mcpb",
      "fileSha256": "${DARWIN_ARM64_SHA}",
      "transport": {
        "type": "stdio"
      }
    },
    {
      "registryType": "mcpb",
      "identifier": "https://github.com/klarlabs-studio/coverctl/releases/download/v${VERSION}/coverctl-mcp-darwin-amd64.mcpb",
      "fileSha256": "${DARWIN_AMD64_SHA}",
      "transport": {
        "type": "stdio"
      }
    },
    {
      "registryType": "mcpb",
      "identifier": "https://github.com/klarlabs-studio/coverctl/releases/download/v${VERSION}/coverctl-mcp-linux-amd64.mcpb",
      "fileSha256": "${LINUX_AMD64_SHA}",
      "transport": {
        "type": "stdio"
      }
    },
    {
      "registryType": "mcpb",
      "identifier": "https://github.com/klarlabs-studio/coverctl/releases/download/v${VERSION}/coverctl-mcp-linux-arm64.mcpb",
      "fileSha256": "${LINUX_ARM64_SHA}",
      "transport": {
        "type": "stdio"
      }
    },
    {
      "registryType": "mcpb",
      "identifier": "https://github.com/klarlabs-studio/coverctl/releases/download/v${VERSION}/coverctl-mcp-windows-amd64.mcpb",
      "fileSha256": "${WINDOWS_AMD64_SHA}",
      "transport": {
        "type": "stdio"
      }
    },
    {
      "registryType": "mcpb",
      "identifier": "https://github.com/klarlabs-studio/coverctl/releases/download/v${VERSION}/coverctl-mcp-windows-arm64.mcpb",
      "fileSha256": "${WINDOWS_ARM64_SHA}",
      "transport": {
        "type": "stdio"
      }
    }
  ]
}
EOF

echo "Updated server.json:"
echo "  Version: ${VERSION}"
echo "  darwin-arm64: ${DARWIN_ARM64_SHA:0:16}..."
echo "  darwin-amd64: ${DARWIN_AMD64_SHA:0:16}..."
echo "  linux-amd64:  ${LINUX_AMD64_SHA:0:16}..."
echo "  linux-arm64:  ${LINUX_ARM64_SHA:0:16}..."
echo "  windows-amd64: ${WINDOWS_AMD64_SHA:0:16}..."
echo "  windows-arm64: ${WINDOWS_ARM64_SHA:0:16}..."
