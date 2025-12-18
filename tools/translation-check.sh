#!/usr/bin/env bash
set -euo pipefail

# Run the translation validation tools from the repository root so paths remain stable.
REPO_ROOT=$(cd "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$REPO_ROOT"

LOCALE_DIR=${1:-options/locale}
LOCALE_FILES=("$LOCALE_DIR"/*.json)

for file in "${LOCALE_FILES[@]}"; do
  if ! pnpm exec jsonlint -q "$file" >/dev/null 2>&1; then
    echo "Invalid JSON syntax: $file"
    exit 1
  fi
  if ! pnpm exec find-duplicated-property-keys -s "$file" >/dev/null 2>&1; then
    echo "Duplicate key found in: $file"
    exit 1
  fi
done
go
