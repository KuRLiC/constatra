#!/usr/bin/env bash
# shellcheck disable=SC2034 # Variables in this file are consumed by sourcing scripts.

# Shared, intentionally small bootstrap configuration.
CONSTATRA_GO_VERSION="1.27.4"

CONSTATRA_SCRIPT_DIR="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
CONSTATRA_REPO_ROOT="$(CDPATH='' cd -- "${CONSTATRA_SCRIPT_DIR}/.." && pwd)"

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "Constatra setup scripts must run on Linux or WSL." >&2
  exit 1
fi

case "$(uname -m)" in
  x86_64|amd64)
    CONSTATRA_HOST_ARCH="amd64"
    CONSTATRA_GO_SHA256="7226736c2339719cda28fbf3a54d71aa63bcefe376e98cef00034268dab24840"
    ;;
  aarch64|arm64)
    CONSTATRA_HOST_ARCH="arm64"
    CONSTATRA_GO_SHA256="3392582988a273a87ab5043ee1b2c9c17a2c3c00580bf9326c47a2e0aaaeac92"
    ;;
  *)
    echo "Unsupported Linux architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

CONSTATRA_TOOLS_ROOT="${CONSTATRA_REPO_ROOT}/.tools"
CONSTATRA_GO_ROOT="${CONSTATRA_TOOLS_ROOT}/go-${CONSTATRA_GO_VERSION}-linux-${CONSTATRA_HOST_ARCH}"
CONSTATRA_GO_BIN="${CONSTATRA_GO_ROOT}/bin/go"
CONSTATRA_BUILD_DIR="${CONSTATRA_REPO_ROOT}/build"

# Keep caches inside the ignored repository tool directory. This makes setup
# self-contained and avoids depending on a writable global Go configuration.
export GOCACHE="${CONSTATRA_TOOLS_ROOT}/cache/go-build"
export GOMODCACHE="${CONSTATRA_TOOLS_ROOT}/cache/go-mod"
export GOTOOLCHAIN="local"
