#!/bin/bash
set -euo pipefail

VERSION="${1:?Usage: $0 <version> (e.g. v0.8.0)}"
REPO="eranchetz/termiNAT"
DIST="dist/${VERSION}"
LDFLAGS="-s -w -X main.Version=${VERSION}"
NOTES_FILE="RELEASE_NOTES_${VERSION}.md"

echo "🚀 Building termiNATor ${VERSION}"
echo "================================"

# Clean and prepare
rm -rf "${DIST}"
mkdir -p "${DIST}"

# Build all platforms
PLATFORMS=("darwin/amd64" "darwin/arm64" "linux/amd64" "linux/arm64" "windows/amd64")
for platform in "${PLATFORMS[@]}"; do
  os="${platform%/*}"
  arch="${platform#*/}"
  output="${DIST}/terminat-${os}-${arch}"
  [ "${os}" = "windows" ] && output="${output}.exe"
  echo "📦 Building ${os}/${arch}..."
  GOOS="${os}" GOARCH="${arch}" go build -ldflags="${LDFLAGS}" -o "${output}" .
done

echo ""
echo "✅ Builds complete:"
ls -lh "${DIST}/"

# Verify version in local binary
LOCAL_ARCH="$(uname -m)"
[ "${LOCAL_ARCH}" = "x86_64" ] && LOCAL_ARCH="amd64"
[ "${LOCAL_ARCH}" = "aarch64" ] && LOCAL_ARCH="arm64"
LOCAL_BIN="${DIST}/terminat-$(uname -s | tr '[:upper:]' '[:lower:]')-${LOCAL_ARCH}"
if [ -f "${LOCAL_BIN}" ]; then
  GOT=$(${LOCAL_BIN} --version 2>&1)
  echo ""
  echo "🔍 Version check: ${GOT}"
  if ! echo "${GOT}" | grep -q "${VERSION}"; then
    echo "❌ Version mismatch! Expected ${VERSION}"
    exit 1
  fi
fi

# Create release and upload
echo ""
if gh release view "${VERSION}" --repo "${REPO}" &>/dev/null; then
  echo "📤 Uploading to existing release ${VERSION}..."
else
  echo "📝 Creating release ${VERSION}..."
  if [ -f "${NOTES_FILE}" ]; then
    echo "🗒️  Using release notes from ${NOTES_FILE}"
    gh release create "${VERSION}" --repo "${REPO}" \
      --title "termiNATor ${VERSION}" \
      --notes-file "${NOTES_FILE}"
  else
    echo "🗒️  No curated release notes found; falling back to generated notes"
    gh release create "${VERSION}" --repo "${REPO}" \
      --title "termiNATor ${VERSION}" \
      --generate-notes
  fi
fi

gh release upload "${VERSION}" --repo "${REPO}" --clobber "${DIST}"/terminat-*
echo ""
echo "✅ Released: https://github.com/${REPO}/releases/tag/${VERSION}"
