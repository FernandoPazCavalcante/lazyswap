#!/usr/bin/env bash
# Cross-compile release tarballs for every supported target.
# Output: dist/lazyswap-<target>.tar.gz (+ .sha256), binary named `lazyswap`.
# Pure-Go build (CGO_ENABLED=0) — go-ethereum + modernc/sqlite cross-compile clean.
#
# Usage: build-release.sh [version]
#
# The version is stamped into the binary, so `lazyswap version` and the MCP
# server handshake report it. semantic-release passes the tag it is about to
# publish; without an argument we fall back to `git describe`, which is right
# for a local build and never silently claims a release.
set -euo pipefail

BIN=lazyswap
OUT=dist
VERSION="${1:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
LDFLAGS="-s -w -X github.com/FernandoPazCavalcante/lazyswap/internal/cli.version=${VERSION}"
mkdir -p "$OUT"

# target-name : GOOS : GOARCH   (names match install.sh's os_id-arch_id)
targets="
linux-x64:linux:amd64
linux-arm64:linux:arm64
darwin-x64:darwin:amd64
darwin-arm64:darwin:arm64
"

for t in $targets; do
  name="${t%%:*}"; rest="${t#*:}"; goos="${rest%%:*}"; goarch="${rest##*:}"
  echo "building ${name} (${goos}/${goarch}) as ${VERSION}"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags "$LDFLAGS" -o "$OUT/$BIN" .
  tar -C "$OUT" -czf "$OUT/lazyswap-${name}.tar.gz" "$BIN"
  rm "$OUT/$BIN"
  ( cd "$OUT" && sha256sum "lazyswap-${name}.tar.gz" > "lazyswap-${name}.tar.gz.sha256" )
done

echo "---"
ls -la "$OUT"
