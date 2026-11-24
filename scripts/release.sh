#!/usr/bin/env bash
set -euo pipefail

CONSTATRA_RELEASE_SCRIPT_DIR="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091 # Resolved relative to this script at runtime.
source "${CONSTATRA_RELEASE_SCRIPT_DIR}/lib.sh"

CONSTATRA_DRY_RUN=false
if [[ "${1:-}" == "--dry-run" ]]; then
  CONSTATRA_DRY_RUN=true
  shift
elif [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  cat <<'EOF'
Usage: ./scripts/release.sh [--dry-run]

Create a versioned Linux/Windows build, checksum manifest, release commit,
annotated tag, and GitHub Release. The only pre-existing tracked modification
allowed is VERSION, which normal local builds advance.

--dry-run validates local prerequisites and prints the next release version
without building, committing, tagging, pushing, or creating a release.
EOF
  exit 0
fi

if [[ $# -ne 0 ]]; then
  echo "Unknown release option: $1" >&2
  exit 2
fi

for CONSTATRA_RELEASE_COMMAND in git gh sha256sum; do
  if ! command -v "${CONSTATRA_RELEASE_COMMAND}" >/dev/null 2>&1; then
    echo "Required release command not found: ${CONSTATRA_RELEASE_COMMAND}" >&2
    exit 1
  fi
done

cd "${CONSTATRA_REPO_ROOT}"

if [[ "$(git branch --show-current)" != "main" ]]; then
  echo "Releases must be created from the main branch." >&2
  exit 1
fi

if ! git diff --cached --quiet; then
  echo "The Git index contains staged changes. Commit or unstage them first." >&2
  exit 1
fi

CONSTATRA_UNRELATED_CHANGES=false
while IFS= read -r CONSTATRA_STATUS_LINE; do
  [[ -z "${CONSTATRA_STATUS_LINE}" ]] && continue
  CONSTATRA_STATUS_PATH="${CONSTATRA_STATUS_LINE:3}"
  if [[ "${CONSTATRA_STATUS_PATH}" != "VERSION" ]]; then
    echo "Release blocked by tracked or untracked change: ${CONSTATRA_STATUS_PATH}" >&2
    CONSTATRA_UNRELATED_CHANGES=true
  fi
done < <(git status --porcelain --untracked-files=all)
if [[ "${CONSTATRA_UNRELATED_CHANGES}" == true ]]; then
  exit 1
fi

CONSTATRA_PLANNED_VERSION="$("${CONSTATRA_RELEASE_SCRIPT_DIR}/version.sh" next)"
CONSTATRA_PLANNED_TAG="v${CONSTATRA_PLANNED_VERSION}"

if [[ "${CONSTATRA_DRY_RUN}" == true ]]; then
  echo "Release preflight passed."
  echo "Current version: $("${CONSTATRA_RELEASE_SCRIPT_DIR}/version.sh" current)"
  echo "Planned version: ${CONSTATRA_PLANNED_VERSION}"
  echo "Planned tag: ${CONSTATRA_PLANNED_TAG}"
  echo "No files, commits, tags, remote refs, or releases were changed."
  exit 0
fi

gh auth status >/dev/null
git fetch origin main

CONSTATRA_LOCAL_HEAD="$(git rev-parse HEAD)"
CONSTATRA_REMOTE_HEAD="$(git rev-parse origin/main)"
if [[ "${CONSTATRA_LOCAL_HEAD}" != "${CONSTATRA_REMOTE_HEAD}" ]]; then
  echo "Local main and origin/main are not synchronized." >&2
  echo "Local:  ${CONSTATRA_LOCAL_HEAD}" >&2
  echo "Remote: ${CONSTATRA_REMOTE_HEAD}" >&2
  exit 1
fi

if git rev-parse --verify --quiet "refs/tags/${CONSTATRA_PLANNED_TAG}" >/dev/null; then
  echo "Tag already exists locally: ${CONSTATRA_PLANNED_TAG}" >&2
  exit 1
fi
if git ls-remote --exit-code --tags origin "refs/tags/${CONSTATRA_PLANNED_TAG}" >/dev/null 2>&1; then
  echo "Tag already exists remotely: ${CONSTATRA_PLANNED_TAG}" >&2
  exit 1
fi

"${CONSTATRA_RELEASE_SCRIPT_DIR}/build.sh" --all
CONSTATRA_RELEASE_VERSION="$("${CONSTATRA_RELEASE_SCRIPT_DIR}/version.sh" current)"
CONSTATRA_RELEASE_TAG="v${CONSTATRA_RELEASE_VERSION}"
if [[ "${CONSTATRA_RELEASE_TAG}" != "${CONSTATRA_PLANNED_TAG}" ]]; then
  echo "Version changed unexpectedly during release preparation." >&2
  exit 1
fi

"${CONSTATRA_GO_BIN}" test -race ./...

(
  cd "${CONSTATRA_BUILD_DIR}"
  sha256sum constatra-linux-amd64 constatra-windows-amd64.exe > SHA256SUMS
)

git add VERSION
git commit \
  -m "Release ${CONSTATRA_RELEASE_TAG}" \
  -m "Record build ${CONSTATRA_RELEASE_VERSION}, validate the race-enabled test suite, and produce matching standalone Linux AMD64 and Windows AMD64 release artifacts with SHA-256 checksums."
git tag -a "${CONSTATRA_RELEASE_TAG}" -m "Constatra ${CONSTATRA_RELEASE_TAG}"

git push --atomic origin main "${CONSTATRA_RELEASE_TAG}"

gh release create "${CONSTATRA_RELEASE_TAG}" \
  "${CONSTATRA_BUILD_DIR}/constatra-linux-amd64" \
  "${CONSTATRA_BUILD_DIR}/constatra-windows-amd64.exe" \
  "${CONSTATRA_BUILD_DIR}/SHA256SUMS" \
  --verify-tag \
  --title "Constatra ${CONSTATRA_RELEASE_TAG}" \
  --generate-notes

echo "Published GitHub Release ${CONSTATRA_RELEASE_TAG}."
