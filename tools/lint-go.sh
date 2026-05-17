#!/usr/bin/env bash
# Builds tools/custom-gcl on demand, then runs it.
set -euo pipefail
cd "$(dirname -- "${BASH_SOURCE[0]}")/.."

GOLANGCI_LINT_VERSION="${GOLANGCI_LINT_PACKAGE##*@}"
BIN="tools/bin/custom-gcl"

NEEDS_BUILD=false
if [ ! -x "$BIN" ]; then
  NEEDS_BUILD=true
elif [ -n "$(find tools/customlint -type f -newer "$BIN" -print -quit)" ]; then
  NEEDS_BUILD=true
elif ! "$BIN" version 2>/dev/null | grep -qF -- "$GOLANGCI_LINT_VERSION-custom-gcl"; then
  NEEDS_BUILD=true
fi

if $NEEDS_BUILD; then
  cat > .custom-gcl.yml <<EOF
version: '$GOLANGCI_LINT_VERSION'
destination: tools/bin
plugins:
  - module: code.gitea.io/gitea
    path: .
    import: code.gitea.io/gitea/tools/customlint
EOF
  env -u GOOS -u GOARCH "$GO" run "$GOLANGCI_LINT_PACKAGE" custom
fi

GOOS=linux "$BIN" run --build-tags=linux,bindata "$@"
if [ -n "${CI:-}" ]; then
  GOOS=windows "$BIN" run --build-tags=windows,gogit "$@"
fi
