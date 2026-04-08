#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALLER_DIR="$ROOT_DIR/installer"
VERSION="${VERSION:-dev}"

targets=(
  "linux amd64 linux/amd64"
  "linux arm64 linux/arm64"
  "darwin amd64 macos/amd64"
  "darwin arm64 macos/arm64"
  "windows amd64 windows/amd64"
  "linux arm64 termux/arm64"
)

log() {
  printf '[uc-package] %s\n' "$*"
}

build_target() {
  local goos="$1"
  local goarch="$2"
  local relative_dir="$3"
  local output_dir="$INSTALLER_DIR/$relative_dir"
  local binary_name="universal-controller"
  local archive_name="universal-controller_${VERSION}_${goos}_${goarch}"

  mkdir -p "$output_dir"
  rm -f "$output_dir"/universal-controller "$output_dir"/universal-controller.exe "$output_dir"/*.tar.gz "$output_dir"/*.zip
  if [[ "$goos" == "windows" ]]; then
    binary_name="${binary_name}.exe"
  fi

  log "building $goos/$goarch -> $relative_dir"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build -o "$output_dir/$binary_name" ./cmd/universal-controller

  if [[ "$goos" == "windows" ]]; then
    (
      cd "$output_dir"
      zip -q -r "${archive_name}.zip" "$binary_name"
    )
  else
    tar -czf "$output_dir/${archive_name}.tar.gz" -C "$output_dir" "$binary_name"
  fi
}

write_checksums() {
  local output_file="$INSTALLER_DIR/checksums.sha256"
  (
    cd "$INSTALLER_DIR"
    while IFS= read -r -d '' file; do
      sha256sum "${file#./}"
    done < <(find . \( -name '*.tar.gz' -o -name '*.zip' \) -print0 | sort -z)
  ) >"$output_file"
}

prepare_dir() {
  mkdir -p \
    "$INSTALLER_DIR/linux/amd64" \
    "$INSTALLER_DIR/linux/arm64" \
    "$INSTALLER_DIR/macos/amd64" \
    "$INSTALLER_DIR/macos/arm64" \
    "$INSTALLER_DIR/windows/amd64" \
    "$INSTALLER_DIR/termux/arm64"
}

cd "$ROOT_DIR"
go test ./...
prepare_dir
for target in "${targets[@]}"; do
  read -r goos goarch relative_dir <<<"$target"
  build_target "$goos" "$goarch" "$relative_dir"
done
write_checksums
log "installer artifacts updated under $INSTALLER_DIR"
