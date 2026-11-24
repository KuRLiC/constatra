#!/usr/bin/env bash
set -euo pipefail

CONSTATRA_VERSION_SCRIPT_DIR="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091 # Resolved relative to this script at runtime.
source "${CONSTATRA_VERSION_SCRIPT_DIR}/lib.sh"

CONSTATRA_VERSION_FILE="${CONSTATRA_REPO_ROOT}/VERSION"

read_version() {
  if [[ ! -f "${CONSTATRA_VERSION_FILE}" ]]; then
    echo "VERSION file not found: ${CONSTATRA_VERSION_FILE}" >&2
    exit 1
  fi

  CONSTATRA_VERSION_VALUE="$(tr -d '[:space:]' < "${CONSTATRA_VERSION_FILE}")"
  if [[ ! "${CONSTATRA_VERSION_VALUE}" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Invalid VERSION value: ${CONSTATRA_VERSION_VALUE}" >&2
    echo "Expected major.minor.patch.build, for example 0.1.0.42" >&2
    exit 1
  fi
  printf '%s\n' "${CONSTATRA_VERSION_VALUE}"
}

next_version() {
  CONSTATRA_CURRENT_VERSION="$(read_version)"
  IFS=. read -r CONSTATRA_MAJOR CONSTATRA_MINOR CONSTATRA_PATCH CONSTATRA_BUILD <<< "${CONSTATRA_CURRENT_VERSION}"
  printf '%s.%s.%s.%s\n' \
    "${CONSTATRA_MAJOR}" "${CONSTATRA_MINOR}" "${CONSTATRA_PATCH}" "$((CONSTATRA_BUILD + 1))"
}

increment_version() {
  mkdir -p "${CONSTATRA_TOOLS_ROOT}"
  CONSTATRA_VERSION_LOCK="${CONSTATRA_TOOLS_ROOT}/version.lock"
  if ! mkdir "${CONSTATRA_VERSION_LOCK}" 2>/dev/null; then
    echo "Another build is updating VERSION, or a stale lock exists:" >&2
    echo "  ${CONSTATRA_VERSION_LOCK}" >&2
    echo "Remove the lock only when no other build is running." >&2
    exit 1
  fi
  trap 'rmdir "${CONSTATRA_VERSION_LOCK}" 2>/dev/null || true' EXIT

  CONSTATRA_NEXT_VERSION="$(next_version)"
  CONSTATRA_VERSION_TEMP="${CONSTATRA_TOOLS_ROOT}/VERSION.next"
  printf '%s\n' "${CONSTATRA_NEXT_VERSION}" > "${CONSTATRA_VERSION_TEMP}"
  mv "${CONSTATRA_VERSION_TEMP}" "${CONSTATRA_VERSION_FILE}"
  printf '%s\n' "${CONSTATRA_NEXT_VERSION}"
}

case "${1:-current}" in
  current)
    read_version
    ;;
  next)
    next_version
    ;;
  increment)
    increment_version
    ;;
  *)
    echo "Usage: ./scripts/version.sh [current|next|increment]" >&2
    exit 2
    ;;
esac
