#!/bin/bash
set -euo pipefail

CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-docker}"
"$CONTAINER_RUNTIME" run --rm --workdir /mnt -v "$PWD:/mnt:ro" "$SHELLCHECK_IMAGE" --color=always "$@"
