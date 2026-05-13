#!/bin/bash
set -euo pipefail

SHELLCHECK_VERSION="${SHELLCHECK_PACKAGE##*@}"
SHELLCHECK="tools/bin/shellcheck-$SHELLCHECK_VERSION"

if [ ! -x "$SHELLCHECK" ]; then
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
  ARCH=$(uname -m)
  [ "$ARCH" = "arm64" ] && ARCH=aarch64 # macOS reports arm64, upstream tarballs use aarch64
  mkdir -p tools/bin
  URL="https://github.com/koalaman/shellcheck/releases/download/$SHELLCHECK_VERSION/shellcheck-$SHELLCHECK_VERSION.$OS.$ARCH.tar.gz"
  curl -fsSL "$URL" | tar -xzO "shellcheck-$SHELLCHECK_VERSION/shellcheck" > "$SHELLCHECK"
  chmod +x "$SHELLCHECK"
fi

exec "$SHELLCHECK" --color=always "$@"
