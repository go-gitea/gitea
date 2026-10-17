#!/bin/bash
set -euo pipefail

# playwright only supports ubuntu/debian officially, and on CI VMs its system deps are pre-installed
if [ -z "${GITHUB_ACTIONS:-}" ] && [ "$(uname -s)" = "Linux" ] && grep -qE '^ID(_LIKE)?=.*(ubuntu|debian)' /etc/os-release 2>/dev/null; then
  pnpm exec playwright install --with-deps "$@"
else
  pnpm exec playwright install "$@"
fi
