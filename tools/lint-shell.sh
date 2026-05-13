#!/bin/bash
set -euo pipefail

SHELLCHECK_VERSION="${SHELLCHECK_PACKAGE##*@}"
SHELLCHECK="tools/bin/shellcheck-$SHELLCHECK_VERSION"

if [ ! -x "$SHELLCHECK" ]; then
  mkdir -p tools/bin
  curl -fsSL "https://github.com/koalaman/shellcheck/releases/download/$SHELLCHECK_VERSION/shellcheck-$SHELLCHECK_VERSION.$(uname -s | tr '[:upper:]' '[:lower:]').$(uname -m | sed 's/arm64/aarch64/').tar.gz" \
    | tar -xzO "shellcheck-$SHELLCHECK_VERSION/shellcheck" \
    > "$SHELLCHECK"
  chmod +x "$SHELLCHECK"
fi

exec "$SHELLCHECK" --color=always "$@"
