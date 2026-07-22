#!/bin/bash
set -euo pipefail

CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-docker}"
VERSION=$(echo "$SHELLCHECK_IMAGE" | sed -E 's/.*:v([0-9.]+)@.*/\1/')

if hash shellcheck 2>/dev/null && shellcheck --version | grep -qx "version: $VERSION"; then
  exec shellcheck --color=always "$@"
else
  exec "$CONTAINER_RUNTIME" run --rm -v "$PWD":/mnt -w /mnt "$SHELLCHECK_IMAGE" --color=always "$@"
fi
