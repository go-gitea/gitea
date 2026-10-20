#!/bin/bash
set -euo pipefail

IS_DEBIAN_FAMILY=""
if [ "$(uname -s)" = "Linux" ] && grep -qE '^ID(_LIKE)?=.*(ubuntu|debian)' /etc/os-release 2>/dev/null; then
  IS_DEBIAN_FAMILY=1
fi

# --with-deps installs system packages and needs root: without it playwright falls back to su and
# prompts for the root password, so only use it when that can happen without a prompt.
# On CI VMs the system deps are pre-installed, so GITHUB_ACTIONS keeps skipping them.
WITH_DEPS=""
if [ -z "${GITHUB_ACTIONS:-}" ] && [ -n "$IS_DEBIAN_FAMILY" ]; then
  if [ "$(id -u)" = 0 ] || { command -v sudo >/dev/null 2>&1 && sudo -n true >/dev/null 2>&1; }; then
    WITH_DEPS=1
  fi
fi

if [ -n "$WITH_DEPS" ]; then
  pnpm exec playwright install --with-deps "$@"
else
  pnpm exec playwright install "$@"
  if [ -z "${GITHUB_ACTIONS:-}" ] && [ -n "$IS_DEBIAN_FAMILY" ]; then
    echo "Skipped playwright system dependencies (not root and no passwordless sudo), install them with: pnpm exec playwright install-deps" >&2
  fi
fi
