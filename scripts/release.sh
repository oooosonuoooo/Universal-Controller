#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="$ROOT_DIR/dist"
VERSION="${VERSION:-dev}"

targets=(
  "linux amd64"
  "linux arm64"
  "darwin amd64"
  "darwin arm64"
  "windows amd64"
)

mkdir -p "$DIST_DIR"
cd "$ROOT_DIR"

for target in "${targets[@]}"; do
  read -r goos goarch <<<"$target"
  suffix=""
  if [[ "$goos" == "windows" ]]; then
    suffix=".exe"
  fi
  archive_base="universal-controller_${VERSION}_${goos}_${goarch}"
  binary="$DIST_DIR/$archive_base/universal-controller$suffix"
  mkdir -p "$(dirname "$binary")"
  GOOS="$goos" GOARCH="$goarch" go build -o "$binary" ./cmd/universal-controller
  (
    cd "$DIST_DIR"
    if [[ "$goos" == "windows" ]]; then
      zip -qr "${archive_base}.zip" "$archive_base"
    else
      tar -czf "${archive_base}.tar.gz" "$archive_base"
    fi
  )
done

printf 'release artifacts written to %s\n' "$DIST_DIR"
