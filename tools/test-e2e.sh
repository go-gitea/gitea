#!/bin/bash
set -euo pipefail

# Determine the Gitea server URL, either from E2E_URL env var or from custom/conf/app.ini
if [ -z "${E2E_URL:-}" ]; then
  INI_FILE="custom/conf/app.ini"
  if [ ! -f "$INI_FILE" ]; then
    echo "error: $INI_FILE not found and E2E_URL not set" >&2
    echo "Either start Gitea with a config or set E2E_URL explicitly:" >&2
    echo "  E2E_URL=http://localhost:3000 make test-e2e" >&2
    exit 1
  fi
  ROOT_URL=$(sed -n 's/^ROOT_URL\s*=\s*//p' "$INI_FILE" | tr -d '[:space:]')
  if [ -z "$ROOT_URL" ]; then
    echo "error: ROOT_URL not found in $INI_FILE" >&2
    exit 1
  fi
  E2E_URL="$ROOT_URL"
fi

# Normalize URL: trim trailing slash to avoid double slashes when appending paths
E2E_URL="${E2E_URL%/}"

echo "Using Gitea server: $E2E_URL"

SERVER_PID=""
cleanup() {
  if [ -n "$SERVER_PID" ]; then
    echo "Stopping temporary Gitea server (PID $SERVER_PID)..."
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

# For local development, if no gitea server is running, start a temporary one.
if [ -z "${CI:-}" ] && ! curl -sf --max-time 5 "$E2E_URL" > /dev/null 2>&1; then
  if [ ! -x "./$EXECUTABLE" ]; then
    echo "error: ./$EXECUTABLE not found or not executable, run 'make backend' first" >&2
    exit 1
  fi
  echo "Starting temporary Gitea server..."
  if [ -n "${E2E_DEBUG:-}" ]; then
    "./$EXECUTABLE" web &
  else
    "./$EXECUTABLE" web > /dev/null 2>&1 &
  fi
  SERVER_PID=$!
fi

# Verify server is reachable, retry for up to 2 minutes for slow startup
MAX_WAIT=120
ELAPSED=0
while ! curl -sf --max-time 5 "$E2E_URL" > /dev/null 2>&1; do
  if [ -n "$SERVER_PID" ] && ! kill -0 "$SERVER_PID" 2>/dev/null; then
    echo "error: Gitea server process exited unexpectedly" >&2
    exit 1
  fi
  if [ "$ELAPSED" -ge "$MAX_WAIT" ]; then
    echo "error: Gitea server at $E2E_URL is not reachable after ${MAX_WAIT}s" >&2
    exit 1
  fi
  sleep 2
  ELAPSED=$((ELAPSED + 2))
done

# Create e2e test user if it does not already exist
E2E_USER="e2e"
E2E_EMAIL="e2e@test.gitea.io"
E2E_PASSWORD="password"
if ! curl -sf --max-time 5 "$E2E_URL/api/v1/users/$E2E_USER" > /dev/null 2>&1; then
  echo "Creating e2e test user..."
  if "./$EXECUTABLE" admin user create --username "$E2E_USER" --email "$E2E_EMAIL" --password "$E2E_PASSWORD" --must-change-password=false; then
    echo "User '$E2E_USER' created"
  else
    echo "error: failed to create user '$E2E_USER'" >&2
    exit 1
  fi
fi

export E2E_URL
export E2E_USER
export E2E_PASSWORD

pnpm exec playwright test "$@"
