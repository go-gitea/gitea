#!/usr/bin/env bash
# Copyright 2026 The Gitea Authors. All rights reserved.
# SPDX-License-Identifier: MIT
#
# Builds tools/custom-gcl on demand, then runs it.
set -euo pipefail
cd "$(dirname -- "${BASH_SOURCE[0]}")/.."

GOLANGCI_LINT_PACKAGE="${GOLANGCI_LINT_PACKAGE:-$(awk -F' *[:?]= *' '/^GOLANGCI_LINT_PACKAGE/{print $2; exit}' Makefile)}"
GOLANGCI_LINT_VERSION="${GOLANGCI_LINT_PACKAGE##*@}"
BIN="tools/custom-gcl"

NEEDS_BUILD=false
if [ ! -x "$BIN" ]; then
  NEEDS_BUILD=true
elif [ -n "$(find tools/customlint -type f -newer "$BIN" -print -quit)" ]; then
  NEEDS_BUILD=true
elif ! "$BIN" version 2>/dev/null | grep -q "$GOLANGCI_LINT_VERSION"; then
  NEEDS_BUILD=true
fi

if $NEEDS_BUILD; then
  TMP_DIR="$(mktemp -d)"
  trap 'rm -rf "$TMP_DIR"' EXIT
  REPO_ROOT="$(pwd)"
  ESC_REPO_ROOT="${REPO_ROOT//\'/\'\'}"
  cat > "$TMP_DIR/.custom-gcl.yml" <<EOF
version: '$GOLANGCI_LINT_VERSION'
destination: '$ESC_REPO_ROOT/tools'
plugins:
  - module: code.gitea.io/gitea
    path: '$ESC_REPO_ROOT'
    import: code.gitea.io/gitea/tools/customlint
EOF
  (unset GOOS GOARCH && cd "$TMP_DIR" && go run "$GOLANGCI_LINT_PACKAGE" custom)
fi

exec "$BIN" run "$@"
