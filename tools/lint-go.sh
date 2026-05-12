#!/usr/bin/env bash
# Builds tools/custom-gcl on demand, then runs it.
set -euo pipefail
cd "$(dirname -- "${BASH_SOURCE[0]}")/.."

GO="${GO:-go}"
GOLANGCI_LINT_PACKAGE="${GOLANGCI_LINT_PACKAGE:-$(awk -F' *[:?]= *' '/^GOLANGCI_LINT_PACKAGE/{sub(/ +#.*$/,"",$2); print $2; exit}' Makefile)}"
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
  cat > .custom-gcl.yml <<EOF
version: '$GOLANGCI_LINT_VERSION'
destination: tools
plugins:
  - module: code.gitea.io/gitea
    path: .
    import: code.gitea.io/gitea/tools/customlint
EOF
  "$GO" run "$GOLANGCI_LINT_PACKAGE" custom
fi

exec "$BIN" run "$@"
