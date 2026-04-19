#!/bin/bash
set -euo pipefail

wait_for_container() {
    local max_attempts=$1
    local attempt=1
    local wait_time=1
    sleep 5 # give the container some time to start listening.

    while [ $attempt -le $max_attempts ]; do
        if $CONTAINER_RUNTIME logs gitea-e2e-runner 2>&1 | grep -q "Listening on"; then
            echo "Container is ready."
            return 0  # Success
        fi

        if [ $attempt -eq $max_attempts ]; then
            echo "Error: Container did not become ready after $max_attempts attempts."
            return 1  # Failure
        fi

        echo "Attempt $attempt: Container not ready, waiting $wait_time second(s)..."
        sleep $wait_time
        ((attempt++))
        ((wait_time*=2))  # Exponential backoff
    done
}

# Create isolated work directory
WORK_DIR=$(mktemp -d)

# Find a random free port
FREE_PORT=$(node -e "const s=require('net').createServer();s.listen(0,'127.0.0.1',()=>{process.stdout.write(String(s.address().port));s.close()})")

cleanup() {
  $CONTAINER_RUNTIME stop gitea-e2e-runner
  if [ -n "${SERVER_PID:-}" ]; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT
# Start playwright worker
$CONTAINER_RUNTIME run --network=host --name gitea-e2e-runner -d --rm --init -it --workdir /home/pwuser --user pwuser mcr.microsoft.com/playwright:v1.59.1-noble /bin/sh -c "npx -y playwright@1.59.1 run-server --port 4000 --host 0.0.0.0"

if ! wait_for_container 5; then
    exit 1
fi


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

[ui.notification]
EVENT_SOURCE_UPDATE_TIME = 500ms

[log]
MODE = console
LEVEL = Warn

[markup.test-external]
ENABLED = true
FILE_EXTENSIONS = .external
RENDER_COMMAND = cat
IS_INPUT_FILE = false
RENDER_CONTENT_MODE = iframe
EOF

export GITEA_WORK_DIR="$WORK_DIR"
export GITEA_TEST_E2E=true

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

# timeout multiplier to make the tests pass on slow CI runners while using
# factor 1 on a fast local machine like a MacBook Pro M1+
if [ -z "${GITEA_TEST_E2E_TIMEOUT_FACTOR:-}" ]; then
  if [ -n "${CI:-}" ]; then
    GITEA_TEST_E2E_TIMEOUT_FACTOR=4
  else
    GITEA_TEST_E2E_TIMEOUT_FACTOR=1
  fi
fi

export GITEA_TEST_E2E_URL="$E2E_URL"
export GITEA_TEST_E2E_DOMAIN
export GITEA_TEST_E2E_USER
export GITEA_TEST_E2E_PASSWORD
export GITEA_TEST_E2E_EMAIL
export GITEA_TEST_E2E_TIMEOUT_FACTOR

PW_TEST_CONNECT_WS_ENDPOINT=ws://127.0.0.1:4000/ pnpm exec playwright test "$@"
