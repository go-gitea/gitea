#!/bin/bash
set -euo pipefail

CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-docker}"
"$CONTAINER_RUNTIME" run --rm -v "$PWD:/mnt:ro" "$SHELLCHECK_IMAGE" --color=always "$@"
