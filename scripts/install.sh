#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ "${PREFIX:-}" == *termux* || -n "${TERMUX_VERSION:-}" ]]; then
  exec bash "$ROOT_DIR/installer/termux/install.sh" "$@"
fi

case "$(uname -s)" in
  Linux) exec bash "$ROOT_DIR/installer/linux/install.sh" "$@" ;;
  Darwin) exec bash "$ROOT_DIR/installer/macos/install.sh" "$@" ;;
  *)
    printf 'unsupported host OS for scripts/install.sh\n' >&2
    exit 1
    ;;
esac
