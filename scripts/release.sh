#!/bin/sh
# Builds the six deckfold binaries (no CGO, static) into dist/.
# Usage: scripts/release.sh [version]
set -eu
cd "$(dirname "$0")/.."
VERSION="${1:-$(git describe --tags --always 2>/dev/null || echo 0.1.0-dev)}"
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
(cd dist && shasum -a 256 deckfold-* > SHA256SUMS)
echo "dist/SHA256SUMS"
