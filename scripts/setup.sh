#!/usr/bin/env bash
set -euo pipefail

CONSTATRA_SETUP_DIR="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091 # Resolved relative to this script at runtime.
source "${CONSTATRA_SETUP_DIR}/lib.sh"

if [[ -x "${CONSTATRA_GO_BIN}" ]]; then
  CONSTATRA_INSTALLED_VERSION="$(${CONSTATRA_GO_BIN} version)"
  if [[ "${CONSTATRA_INSTALLED_VERSION}" == *"go${CONSTATRA_GO_VERSION} "* ]]; then
    echo "Go ${CONSTATRA_GO_VERSION} is already ready at ${CONSTATRA_GO_ROOT}"
    exit 0
  fi
fi

for CONSTATRA_REQUIRED_COMMAND in curl tar sha256sum mktemp; do
  if ! command -v "${CONSTATRA_REQUIRED_COMMAND}" >/dev/null 2>&1; then
    echo "Required command not found: ${CONSTATRA_REQUIRED_COMMAND}" >&2
    exit 1
  fi
done

if [[ -e "${CONSTATRA_GO_ROOT}" ]]; then
  echo "The tool directory exists but does not contain the expected Go version:" >&2
  echo "  ${CONSTATRA_GO_ROOT}" >&2
  echo "Move or remove that directory, then run setup again." >&2
  exit 1
fi

mkdir -p "${CONSTATRA_TOOLS_ROOT}"
CONSTATRA_TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/constatra-setup.XXXXXX")"
trap 'rm -rf -- "${CONSTATRA_TEMP_DIR}"' EXIT

CONSTATRA_ARCHIVE="go${CONSTATRA_GO_VERSION}.linux-${CONSTATRA_HOST_ARCH}.tar.gz"
CONSTATRA_ARCHIVE_PATH="${CONSTATRA_TEMP_DIR}/${CONSTATRA_ARCHIVE}"
CONSTATRA_DOWNLOAD_URL="https://go.dev/dl/${CONSTATRA_ARCHIVE}"

echo "Downloading Go ${CONSTATRA_GO_VERSION} for linux/${CONSTATRA_HOST_ARCH}..."
curl --fail --location --retry 3 --output "${CONSTATRA_ARCHIVE_PATH}" "${CONSTATRA_DOWNLOAD_URL}"

echo "Verifying the official SHA-256 checksum..."
printf '%s  %s\n' "${CONSTATRA_GO_SHA256}" "${CONSTATRA_ARCHIVE_PATH}" | sha256sum --check --status

tar -xzf "${CONSTATRA_ARCHIVE_PATH}" -C "${CONSTATRA_TEMP_DIR}"
mv "${CONSTATRA_TEMP_DIR}/go" "${CONSTATRA_GO_ROOT}"
mkdir -p "${GOCACHE}" "${GOMODCACHE}"

echo "Setup complete."
echo "Go: $(${CONSTATRA_GO_BIN} version)"
echo "Location: ${CONSTATRA_GO_ROOT}"
