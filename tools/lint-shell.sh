#!/bin/bash
set -euo pipefail

: "${SHELLCHECK_IMAGE:?must be set}"
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-docker}"

version=$(echo "$SHELLCHECK_IMAGE" | sed -E 's/.*:v([0-9.]+)@.*/\1/')

if hash shellcheck 2>/dev/null && shellcheck --version | grep -qx "version: $version"; then
  exec shellcheck --color=always "$@"
else
  exec "$CONTAINER_RUNTIME" run --rm -v "$PWD":/mnt -w /mnt "$SHELLCHECK_IMAGE" --color=always "$@"
fi
