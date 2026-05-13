#!/bin/bash
set -euo pipefail

SHELLCHECK_VERSION="${SHELLCHECK_PACKAGE##*@}"
SHELLCHECK="tools/bin/shellcheck-$SHELLCHECK_VERSION"

if [ ! -x "$SHELLCHECK" ]; then
  mkdir -p tools/bin
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
  ARCH=$(uname -m | sed 's/arm64/aarch64/')
  URL="https://github.com/koalaman/shellcheck/releases/download/$SHELLCHECK_VERSION"
  URL="$URL/shellcheck-$SHELLCHECK_VERSION.$OS.$ARCH.tar.gz"
  curl -fsSL "$URL" | tar -xzO "shellcheck-$SHELLCHECK_VERSION/shellcheck" > "$SHELLCHECK"
  chmod +x "$SHELLCHECK"
fi

exec "$SHELLCHECK" --color=always "$@"
