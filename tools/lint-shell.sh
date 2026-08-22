#!/bin/bash
set -euo pipefail

CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-docker}"
VERSION=$(echo "$SHELLCHECK_IMAGE" | sed -E 's/.*:v([0-9.]+)@.*/\1/')
EXCLUDED_RULES="SC2153" # False-alert "Possible misspelling: TAGS may not be assigned. Did you mean tags?". We already use strict mode.

if hash shellcheck 2>/dev/null && shellcheck --version | grep -qx "version: $VERSION"; then
  exec shellcheck --color=always -e "$EXCLUDED_RULES" "$@"
else
  exec "$CONTAINER_RUNTIME" run --rm -v "$PWD":/mnt -w /mnt "$SHELLCHECK_IMAGE" --color=always -e "$EXCLUDED_RULES" "$@"
fi
