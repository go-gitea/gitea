#!/bin/bash
set -euo pipefail

# Determine the Gitea server URL, either from GITEA_URL env var or from custom/conf/app.ini
if [ -n "${GITEA_URL:-}" ]; then
  GITEA_TEST_SERVER_URL="$GITEA_URL"
else
  INI_FILE="custom/conf/app.ini"
  if [ ! -f "$INI_FILE" ]; then
    echo "error: $INI_FILE not found and GITEA_URL not set" >&2
    echo "Either start Gitea with a config or set GITEA_URL explicitly:" >&2
    echo "  GITEA_URL=http://localhost:3000 make test-e2e" >&2
    exit 1
  fi
  ROOT_URL=$(sed -n 's/^ROOT_URL\s*=\s*//p' "$INI_FILE" | tr -d '[:space:]')
  if [ -z "$ROOT_URL" ]; then
    echo "error: ROOT_URL not found in $INI_FILE" >&2
    exit 1
  fi
  GITEA_TEST_SERVER_URL="$ROOT_URL"
fi

echo "Using Gitea server: $GITEA_TEST_SERVER_URL"

# Verify server is reachable
if ! curl -sf --max-time 5 "$GITEA_TEST_SERVER_URL" > /dev/null 2>&1; then
  echo "error: Gitea server at $GITEA_TEST_SERVER_URL is not reachable" >&2
  echo "Start Gitea first: ${EXECUTABLE:-./gitea}" >&2
  exit 1
fi

# Create e2e test user if it does not already exist
E2E_USER="e2e"
E2E_EMAIL="e2e@test.gitea.io"
E2E_PASSWORD="password"
if ! curl -sf --max-time 5 "$GITEA_TEST_SERVER_URL/api/v1/users/$E2E_USER" > /dev/null 2>&1; then
  echo "Creating e2e test user..."
  if ${EXECUTABLE:-./gitea} admin user create --username "$E2E_USER" --email "$E2E_EMAIL" --password "$E2E_PASSWORD" --must-change-password=false 2>/dev/null; then
    echo "User '$E2E_USER' created"
  else
    echo "error: failed to create user '$E2E_USER'" >&2
    exit 1
  fi
fi

export GITEA_TEST_SERVER_URL
exec pnpm exec playwright test "$@"
