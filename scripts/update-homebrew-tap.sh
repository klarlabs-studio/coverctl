#!/bin/bash
# Update Homebrew tap with new release
set -euo pipefail

VERSION="${1:-}"
HOMEBREW_TAP_TOKEN="${HOMEBREW_TAP_TOKEN:-}"

if [ -z "$VERSION" ]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 1.10.0"
    exit 1
fi

if [ -z "$HOMEBREW_TAP_TOKEN" ]; then
    echo "Error: HOMEBREW_TAP_TOKEN environment variable not set"
    exit 1
fi

# Calculate SHA256 hashes for each platform
echo "Calculating SHA256 hashes..."
SHA_DARWIN_ARM64=$(sha256sum dist/coverctl-darwin-arm64.tar.gz | awk '{print $1}')
SHA_DARWIN_AMD64=$(sha256sum dist/coverctl-darwin-amd64.tar.gz | awk '{print $1}')
SHA_LINUX_ARM64=$(sha256sum dist/coverctl-linux-arm64.tar.gz | awk '{print $1}')
SHA_LINUX_AMD64=$(sha256sum dist/coverctl-linux-amd64.tar.gz | awk '{print $1}')

echo "SHA256 hashes:"
echo "  darwin-arm64: $SHA_DARWIN_ARM64"
echo "  darwin-amd64: $SHA_DARWIN_AMD64"
echo "  linux-arm64:  $SHA_LINUX_ARM64"
echo "  linux-amd64:  $SHA_LINUX_AMD64"

# Clone homebrew-tap (use credential store to avoid embedding token in URL)
echo "Cloning homebrew-tap..."
rm -rf tap
CRED_FILE=$(mktemp)
trap 'rm -f "$CRED_FILE"' EXIT
echo "https://x-access-token:${HOMEBREW_TAP_TOKEN}@github.com" > "$CRED_FILE"
git config --global credential.helper "store --file=${CRED_FILE}"
git clone "https://github.com/felixgeelhaar/homebrew-tap.git" tap
cd tap

# Generate updated formula
echo "Generating formula..."
cat > Formula/coverctl.rb << EOF
# Homebrew formula for Coverctl
# To install: brew tap felixgeelhaar/tap && brew install coverctl
class Coverctl < Formula
  desc "Declarative, domain-aware coverage enforcement for any language"
  homepage "https://github.com/klarlabs-studio/coverctl"
  version "${VERSION}"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/klarlabs-studio/coverctl/releases/download/v#{version}/coverctl-darwin-arm64.tar.gz"
      sha256 "${SHA_DARWIN_ARM64}"

      def install
        bin.install "coverctl-darwin-arm64" => "coverctl"
      end
    else
      url "https://github.com/klarlabs-studio/coverctl/releases/download/v#{version}/coverctl-darwin-amd64.tar.gz"
      sha256 "${SHA_DARWIN_AMD64}"

      def install
        bin.install "coverctl-darwin-amd64" => "coverctl"
      end
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/klarlabs-studio/coverctl/releases/download/v#{version}/coverctl-linux-arm64.tar.gz"
      sha256 "${SHA_LINUX_ARM64}"

      def install
        bin.install "coverctl-linux-arm64" => "coverctl"
      end
    else
      url "https://github.com/klarlabs-studio/coverctl/releases/download/v#{version}/coverctl-linux-amd64.tar.gz"
      sha256 "${SHA_LINUX_AMD64}"

      def install
        bin.install "coverctl-linux-amd64" => "coverctl"
      end
    end
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/coverctl --version")
  end
end
EOF

# Commit and push
echo "Committing changes..."
git config user.name "github-actions[bot]"
git config user.email "github-actions[bot]@users.noreply.github.com"
git add Formula/coverctl.rb
git commit -m "coverctl: update to v${VERSION}"
git push

echo "Done! Homebrew tap updated to v${VERSION}"
