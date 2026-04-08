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
SOURCE_DIR="$ROOT_DIR"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
ENABLE_SERVICE=0
TEMP_DIR=""

cleanup() {
  if [[ -n "$TEMP_DIR" && -d "$TEMP_DIR" ]]; then
    rm -rf "$TEMP_DIR"
  fi
}
trap cleanup EXIT

log() {
  printf '[uc-linux-installer] %s\n' "$*" >&2
}

ensure_go() {
  if command -v go >/dev/null 2>&1; then
    return 0
  fi

  bootstrap_go || return 1
  command -v go >/dev/null 2>&1
}

run_with_privilege() {
  if [[ ${EUID:-$(id -u)} -eq 0 ]]; then
    "$@"
  elif command -v sudo >/dev/null 2>&1; then
    sudo "$@"
  else
    "$@"
  fi
}

bootstrap_go() {
  if command -v apt-get >/dev/null 2>&1; then
    log "Go is missing; attempting installation with apt-get"
    if ! run_with_privilege apt-get update; then
      return 1
    fi
    if run_with_privilege apt-get install -y golang-go; then
      return 0
    fi
    if run_with_privilege apt-get install -y golang; then
      return 0
    fi
    return 1
  fi

  if command -v dnf >/dev/null 2>&1; then
    log "Go is missing; attempting installation with dnf"
    if run_with_privilege dnf install -y golang; then
      return 0
    fi
    return 1
  fi

  if command -v yum >/dev/null 2>&1; then
    log "Go is missing; attempting installation with yum"
    if run_with_privilege yum install -y golang; then
      return 0
    fi
    return 1
  fi

  if command -v pacman >/dev/null 2>&1; then
    log "Go is missing; attempting installation with pacman"
    if run_with_privilege pacman -Sy --noconfirm go; then
      return 0
    fi
    return 1
  fi

  if command -v apk >/dev/null 2>&1; then
    log "Go is missing; attempting installation with apk"
    if run_with_privilege apk add go; then
      return 0
    fi
    return 1
  fi

  if command -v zypper >/dev/null 2>&1; then
    log "Go is missing; attempting installation with zypper"
    if run_with_privilege zypper --non-interactive install go; then
      return 0
    fi
    return 1
  fi

  if command -v brew >/dev/null 2>&1; then
    log "Go is missing; attempting installation with brew"
    if brew install go; then
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

for arg in "$@"; do
  case "$arg" in
    --service) ENABLE_SERVICE=1 ;;
    *)
      printf 'unknown argument: %s\n' "$arg" >&2
      exit 1
      ;;
  esac
done

extract_binary_if_needed() {
  local arch_dir="$1"
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
  if [[ ! -f "$ROOT_DIR/go.mod" ]]; then
    printf 'could not locate the repository root at %s\n' "$ROOT_DIR" >&2
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

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) ARCH_DIR="amd64" ;;
  aarch64|arm64) ARCH_DIR="arm64" ;;
  *)
    printf 'unsupported Linux architecture: %s\n' "$arch" >&2
    exit 1
    ;;
esac

if ! BINARY_SOURCE="$(extract_binary_if_needed "$ARCH_DIR")"; then
  BINARY_SOURCE="$(build_from_source)"
fi

mkdir -p "$INSTALL_DIR"
install -m 0755 "$BINARY_SOURCE" "$INSTALL_DIR/universal-controller"
"$INSTALL_DIR/universal-controller" repair || true

if [[ "$ENABLE_SERVICE" -eq 1 ]]; then
  run_with_privilege "$INSTALL_DIR/universal-controller" receiver install-service
  run_with_privilege systemctl daemon-reload
  run_with_privilege systemctl enable --now universal-controller-receiver.service
fi

printf 'installed universal-controller to %s/universal-controller\n' "$INSTALL_DIR"
