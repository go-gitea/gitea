#!/bin/bash
set -euo pipefail

# playwright only supports ubuntu/debian officially, and on CI VMs its system deps are pre-installed
if [ -z "${GITHUB_ACTIONS:-}" ] && [ "$(uname -s)" = "Linux" ] && grep -qE '^ID(_LIKE)?=.*(ubuntu|debian)' /etc/os-release 2>/dev/null; then
  if [ "$(id -u)" = 0 ] || { command -v sudo >/dev/null 2>&1 && sudo -n true >/dev/null 2>&1; }; then # Avoid Playwright's interactive su fallback.
    pnpm exec playwright install --with-deps "$@"
  else
    pnpm exec playwright install "$@"
    echo "Skipped playwright system dependencies (not root and no passwordless sudo), install them with: pnpm exec playwright install-deps" >&2
  fi
else
  pnpm exec playwright install "$@"
fi
