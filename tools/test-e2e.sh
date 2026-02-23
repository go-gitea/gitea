#!/bin/bash
set -euo pipefail

# Create isolated work directory
WORK_DIR=$(mktemp -d)

# Find a random free port
FREE_PORT=$(node -e "const s=require('net').createServer();s.listen(0,'127.0.0.1',()=>{process.stdout.write(String(s.address().port));s.close()})")

cleanup() {
  if [ -n "${SERVER_PID:-}" ]; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

# Write config file for isolated instance
mkdir -p "$WORK_DIR/custom/conf"
cat > "$WORK_DIR/custom/conf/app.ini" <<EOF
[database]
DB_TYPE = sqlite3
PATH = $WORK_DIR/data/gitea.db

[server]
HTTP_PORT = $FREE_PORT
ROOT_URL = http://localhost:$FREE_PORT
STATIC_ROOT_PATH = $(pwd)

[security]
INSTALL_LOCK = true

[service]
ENABLE_CAPTCHA = false

[log]
MODE = console
LEVEL = Warn
EOF

export GITEA_WORK_DIR="$WORK_DIR"

# Start Gitea server
echo "Starting Gitea server on port $FREE_PORT (workdir: $WORK_DIR)..."
if [ -n "${GITEA_TEST_E2E_DEBUG:-}" ]; then
  "./$EXECUTABLE" web &
else
  "./$EXECUTABLE" web > "$WORK_DIR/server.log" 2>&1 &
fi
SERVER_PID=$!

# Wait for server to be reachable
E2E_URL="http://localhost:$FREE_PORT"
MAX_WAIT=120
ELAPSED=0
while ! curl -sf --max-time 5 "$E2E_URL" > /dev/null 2>&1; do
  if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    echo "error: Gitea server process exited unexpectedly. Server log:" >&2
    cat "$WORK_DIR/server.log" 2>/dev/null >&2 || true
    exit 1
  fi
  if [ "$ELAPSED" -ge "$MAX_WAIT" ]; then
    echo "error: Gitea server not reachable after ${MAX_WAIT}s. Server log:" >&2
    cat "$WORK_DIR/server.log" 2>/dev/null >&2 || true
    exit 1
  fi
  sleep 2
  ELAPSED=$((ELAPSED + 2))
done

echo "Gitea server is ready at $E2E_URL"

GITEA_TEST_E2E_DOMAIN="e2e.gitea.com"
GITEA_TEST_E2E_USER="e2e-admin"
GITEA_TEST_E2E_PASSWORD="password"
GITEA_TEST_E2E_EMAIL="$GITEA_TEST_E2E_USER@$GITEA_TEST_E2E_DOMAIN"

# Create admin test user
"./$EXECUTABLE" admin user create \
  --username "$GITEA_TEST_E2E_USER" \
  --password "$GITEA_TEST_E2E_PASSWORD" \
  --email "$GITEA_TEST_E2E_EMAIL" \
  --must-change-password=false \
  --admin

export GITEA_TEST_E2E_URL="$E2E_URL"
export GITEA_TEST_E2E_DOMAIN
export GITEA_TEST_E2E_USER
export GITEA_TEST_E2E_PASSWORD
export GITEA_TEST_E2E_EMAIL

pnpm exec playwright test "$@"
