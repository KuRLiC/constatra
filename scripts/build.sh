#!/usr/bin/env bash
set -euo pipefail

CONSTATRA_BUILD_SCRIPT_DIR="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091 # Resolved relative to this script at runtime.
source "${CONSTATRA_BUILD_SCRIPT_DIR}/lib.sh"

CONSTATRA_BUILD_MODE="native"
if [[ "${1:-}" == "--all" ]]; then
  CONSTATRA_BUILD_MODE="all"
  shift
elif [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  cat <<'EOF'
Usage: ./scripts/build.sh [--all]

Without options, build the Linux binary for the current machine.
With --all, build Linux AMD64 and Windows AMD64 release binaries.

Optional environment:
  CONSTATRA_VERSION   Version embedded in the binaries (default: dev)
EOF
  exit 0
fi

if [[ $# -ne 0 ]]; then
  echo "Unknown build option: $1" >&2
  echo "Run ./scripts/build.sh --help for usage." >&2
  exit 2
fi

if [[ ! -x "${CONSTATRA_GO_BIN}" ]]; then
  "${CONSTATRA_BUILD_SCRIPT_DIR}/setup.sh"
fi

CONSTATRA_VERSION="${CONSTATRA_VERSION:-dev}"
CONSTATRA_LDFLAGS="-s -w -X main.version=${CONSTATRA_VERSION}"
mkdir -p "${CONSTATRA_BUILD_DIR}"

if [[ "${CONSTATRA_BUILD_MODE}" == "all" ]]; then
  echo "Building Linux AMD64..."
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 "${CONSTATRA_GO_BIN}" build \
    -trimpath -ldflags "${CONSTATRA_LDFLAGS}" \
    -o "${CONSTATRA_BUILD_DIR}/constatra-linux-amd64" "${CONSTATRA_REPO_ROOT}/cmd/constatra"

  echo "Building Windows AMD64..."
  CGO_ENABLED=0 GOOS=windows GOARCH=amd64 "${CONSTATRA_GO_BIN}" build \
    -trimpath -ldflags "${CONSTATRA_LDFLAGS}" \
    -o "${CONSTATRA_BUILD_DIR}/constatra-windows-amd64.exe" "${CONSTATRA_REPO_ROOT}/cmd/constatra"

  echo "Build complete:"
  echo "  ${CONSTATRA_BUILD_DIR}/constatra-linux-amd64"
  echo "  ${CONSTATRA_BUILD_DIR}/constatra-windows-amd64.exe"
  exit 0
fi

echo "Building Constàtra for Linux ${CONSTATRA_HOST_ARCH}..."
CGO_ENABLED=0 GOOS=linux GOARCH="${CONSTATRA_HOST_ARCH}" "${CONSTATRA_GO_BIN}" build \
  -trimpath -ldflags "${CONSTATRA_LDFLAGS}" \
  -o "${CONSTATRA_BUILD_DIR}/constatra" "${CONSTATRA_REPO_ROOT}/cmd/constatra"
echo "Build complete: ${CONSTATRA_BUILD_DIR}/constatra"
