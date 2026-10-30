#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../../gsso/go"
rm -rf dist
mkdir dist

for os in darwin linux windows; do
  for arch in amd64 arm64; do
    ext=""
    if [ "$os" = windows ]; then ext=".exe"; fi
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
      go build -trimpath -ldflags "-s -w" -o "dist/gsso-$os-$arch$ext" ./cmd/gsso
  done
done
