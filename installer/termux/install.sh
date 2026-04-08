#!/usr/bin/env bash
set -euo pipefail

resolve_script_dir() {
  local source="${BASH_SOURCE[0]}"
  while [[ -L "$source" ]]; do
    local dir
    dir="$(cd -P "$(dirname "$source")" && pwd)"
    source="$(readlink "$source")"
    [[ "$source" != /* ]] && source="$dir/$source"
  done
  cd -P "$(dirname "$source")" && pwd
}

BASE_DIR="$(resolve_script_dir)"
ROOT_DIR="$(cd "$BASE_DIR/../.." && pwd)"
if [[ -n "${PREFIX:-}" ]]; then
  PREFIX_DIR="$PREFIX"
elif [[ -n "${TERMUX_VERSION:-}" ]]; then
  PREFIX_DIR="/data/data/com.termux/files/usr"
else
  PREFIX_DIR="$HOME/.local"
fi
TEMP_DIR=""
SOURCE_DIR="$ROOT_DIR"
SOURCE_TEMP_DIR=""

DEFAULT_SOURCE_URL="https://github.com/oooosonuoooo/Universal-Controller.git"
if command -v git >/dev/null 2>&1; then
  remote_url="$(git -C "$ROOT_DIR" config --get remote.origin.url 2>/dev/null || true)"
  if [[ -n "$remote_url" ]]; then
    DEFAULT_SOURCE_URL="$remote_url"
  fi
fi
SOURCE_URL="${UC_REPO_URL:-$DEFAULT_SOURCE_URL}"

cleanup() {
  if [[ -n "$TEMP_DIR" && -d "$TEMP_DIR" ]]; then
    rm -rf "$TEMP_DIR"
  fi
  if [[ -n "$SOURCE_TEMP_DIR" && -d "$SOURCE_TEMP_DIR" ]]; then
    rm -rf "$SOURCE_TEMP_DIR"
  fi
}
trap cleanup EXIT

log() {
  printf '[uc-termux-installer] %s\n' "$*" >&2
}

ensure_go() {
  if command -v go >/dev/null 2>&1; then
    return 0
  fi

  bootstrap_go || return 1
  command -v go >/dev/null 2>&1
}

ensure_git() {
  if command -v git >/dev/null 2>&1; then
    return 0
  fi

  if command -v pkg >/dev/null 2>&1; then
    log "git is missing; installing with pkg"
    if pkg install -y git; then
      return 0
    fi
  fi

  command -v git >/dev/null 2>&1
}

bootstrap_go() {
  if command -v pkg >/dev/null 2>&1; then
    log "Go is missing; installing with pkg"
    if pkg update -y && pkg install -y golang; then
      return 0
    fi
    return 1
  fi

  return 1
}

repair_modules() {
  log "repairing Go module cache"
  (
    cd "$SOURCE_DIR"
    go clean -cache -modcache
    go mod download
  )
}

ensure_source_root() {
  if [[ -f "$ROOT_DIR/go.mod" ]]; then
    SOURCE_DIR="$ROOT_DIR"
    return 0
  fi

  if [[ -f "$SOURCE_DIR/go.mod" ]]; then
    return 0
  fi

  if ! ensure_git; then
    printf 'git is required to fetch source when the repository root is not present.\n' >&2
    return 1
  fi

  SOURCE_TEMP_DIR="$(mktemp -d)"
  log "repository root not found; cloning source to a temporary checkout"
  if ! git clone --depth 1 "$SOURCE_URL" "$SOURCE_TEMP_DIR/source"; then
    printf 'failed to clone source from %s\n' "$SOURCE_URL" >&2
    return 1
  fi

  SOURCE_DIR="$SOURCE_TEMP_DIR/source"
  if [[ ! -f "$SOURCE_DIR/go.mod" ]]; then
    printf 'cloned source checkout is missing go.mod at %s\n' "$SOURCE_DIR" >&2
    return 1
  fi
}

ensure_command_entrypoint() {
  local entrypoint_dir="$1/cmd/universal-controller"
  local entrypoint_file="$entrypoint_dir/main.go"
  if [[ -f "$entrypoint_file" ]]; then
    return 0
  fi

  log "restoring missing command entrypoint"
  mkdir -p "$entrypoint_dir"
  cat >"$entrypoint_file" <<'EOF'
package main

import (
	"fmt"
	"os"

	"universal-controller/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
EOF
}

extract_binary_if_needed() {
  local arch_dir="arm64"
  local arch_path="$BASE_DIR/$arch_dir"
  local binary_path="$arch_path/universal-controller"

  if [[ -f "$binary_path" ]]; then
    printf '%s' "$binary_path"
    return 0
  fi

  if [[ ! -d "$arch_path" ]]; then
    return 1
  fi

  local archive
  archive="$(find "$arch_path" -maxdepth 1 -type f -name 'universal-controller_*.tar.gz' -print -quit 2>/dev/null || true)"
  if [[ -z "$archive" ]]; then
    return 1
  fi

  TEMP_DIR="$(mktemp -d)"
  tar -xzf "$archive" -C "$TEMP_DIR"
  if [[ -x "$TEMP_DIR/universal-controller" ]]; then
    printf '%s' "$TEMP_DIR/universal-controller"
    return 0
  fi
  return 1
}

build_from_source() {
  log "attempting to build from source"
  if ! ensure_go; then
    printf 'Go is required to build from source. Install Go or provide packaged installer artifacts.\n' >&2
    exit 1
  fi
  if ! ensure_source_root; then
    exit 1
  fi

  ensure_command_entrypoint "$SOURCE_DIR"

  local build_dir="$SOURCE_DIR/build"
  local binary_path="$build_dir/universal-controller"
  mkdir -p "$build_dir"

  if ! (
    cd "$SOURCE_DIR"
    go mod download
    CGO_ENABLED=0 go build -o "$binary_path" ./cmd/universal-controller
  ); then
    log "retrying build after clearing Go caches"
    repair_modules
    if ! (
      cd "$SOURCE_DIR"
      CGO_ENABLED=0 go build -o "$binary_path" ./cmd/universal-controller
    ); then
      printf 'build failed after retry, see errors above\n' >&2
      exit 1
    fi
  fi

  if [[ ! -x "$binary_path" ]]; then
    printf 'build failed, executable not found at %s\n' "$binary_path" >&2
    exit 1
  fi
  printf '%s' "$binary_path"
}

if ! BINARY_SOURCE="$(extract_binary_if_needed)"; then
  BINARY_SOURCE="$(build_from_source)"
fi

mkdir -p "$PREFIX_DIR/bin"
install -m 0755 "$BINARY_SOURCE" "$PREFIX_DIR/bin/universal-controller"
"$PREFIX_DIR/bin/universal-controller" repair || true
printf 'installed universal-controller to %s/bin/universal-controller\n' "$PREFIX_DIR"

# Ensure the install directory is in PATH
case ":${PATH}:" in
  *":$PREFIX_DIR/bin:"*) ;;
  *)
    printf '\n'
    printf 'NOTE: %s/bin is not in your PATH.\n' "$PREFIX_DIR"
    printf 'Add it by running:\n'
    printf '  export PATH="%s/bin:$PATH"\n' "$PREFIX_DIR"
    printf 'To make it permanent, add the line above to your ~/.bashrc or ~/.zshrc\n'
    ;;
esac
