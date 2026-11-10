#!/bin/sh
# Builds the six deckfold binaries (no CGO, static) into dist/, with the third-party licenses.
# Usage: scripts/release.sh [version]
set -eu
cd "$(dirname "$0")/.."
VERSION="${1:-$(git describe --tags --always 2>/dev/null || echo dev)}"
go generate ./runtime
go test ./... >/dev/null
rm -rf dist
mkdir -p dist
for target in darwin/arm64 darwin/amd64 windows/amd64 windows/arm64 linux/amd64 linux/arm64; do
  os="${target%/*}"; arch="${target#*/}"
  ext=""; [ "$os" = windows ] && ext=".exe"
  out="dist/deckfold-${VERSION}-${os}-${arch}${ext}"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags "-s -w -X main.Version=${VERSION}" -o "$out" ./cmd/deckfold
  echo "$out"
done
cp THIRD_PARTY_LICENSES.md dist/
(cd dist && shasum -a 256 deckfold-* THIRD_PARTY_LICENSES.md > SHA256SUMS)
echo "dist/THIRD_PARTY_LICENSES.md"
echo "dist/SHA256SUMS"
