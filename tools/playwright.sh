#!/bin/bash
set -euo pipefail

args=(install)

# playwright only supports ubuntu/debian officially, and on CI VMs its system deps are pre-installed
if [ -z "${GITHUB_ACTIONS:-}" ] && [ "$(uname -s)" = "Linux" ] && grep -qE '^ID(_LIKE)?=.*(ubuntu|debian)' /etc/os-release 2>/dev/null; then
  args+=(--with-deps)
fi

pnpm exec playwright "${args[@]}" chromium firefox
