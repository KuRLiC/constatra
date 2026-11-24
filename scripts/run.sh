#!/usr/bin/env bash
set -euo pipefail

CONSTATRA_RUN_SCRIPT_DIR="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091 # Resolved relative to this script at runtime.
source "${CONSTATRA_RUN_SCRIPT_DIR}/lib.sh"

# Go's incremental build cache makes rebuilding cheap and guarantees that the
# executable always matches the checked-out source. Each invocation is a build
# and therefore advances the persistent application version once.
"${CONSTATRA_RUN_SCRIPT_DIR}/build.sh"

exec "${CONSTATRA_BUILD_DIR}/constatra" "$@"
