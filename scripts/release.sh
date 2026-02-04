#!/bin/bash
set -e

VERSION="${1:-$(git describe --tags --abbrev=0 2>/dev/null || echo "dev")}"
REPO="eranchetz/termiNAT"

echo "🚀 Building termiNATor $VERSION"
echo "================================"

mkdir -p dist

# Build all platforms
for GOOS in linux darwin; do
  for GOARCH in amd64 arm64; do
    OUTPUT="dist/terminat-${GOOS}-${GOARCH}"
    echo "📦 Building ${GOOS}/${GOARCH}..."
    GOOS=$GOOS GOARCH=$GOARCH go build -o "$OUTPUT" -ldflags "-X main.Version=$VERSION" .
    chmod +x "$OUTPUT"
  done
done

echo ""
echo "✅ All builds complete!"
ls -lh dist/

# Upload to existing release if it exists
if gh release view "$VERSION" --repo "$REPO" &>/dev/null; then
  echo ""
  echo "📤 Uploading binaries to release $VERSION..."
  gh release upload "$VERSION" --repo "$REPO" --clobber \
    dist/terminat-linux-amd64 \
    dist/terminat-linux-arm64 \
    dist/terminat-darwin-amd64 \
    dist/terminat-darwin-arm64
  echo "✅ Binaries uploaded to https://github.com/$REPO/releases/tag/$VERSION"
else
  echo ""
  echo "⚠️  Release $VERSION not found. Create it first with:"
  echo "   gh release create $VERSION --title \"termiNATor $VERSION\""
fi
