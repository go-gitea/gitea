#!/bin/bash
set -euo pipefail

CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-docker}"
CONTAINER_NAME="gitea-e2e-runner-$$"

free_port() {
  node -e "const s=require('net').createServer();s.listen(0,'127.0.0.1',()=>{process.stdout.write(String(s.address().port));s.close()})"
}

detect_playwright_mode() {
  if [ "${PLAYWRIGHT_MODE:-auto}" = "local" ] || [ "${PLAYWRIGHT_MODE:-auto}" = "container" ]; then
    return
  fi

  PLAYWRIGHT_MODE="local"

  if [ "$(uname -s)" = "Linux" ]; then
    # playwright only supports ubuntu/debian officially
    if ! grep -qE '^ID(_LIKE)?=.*(ubuntu|debian)' /etc/os-release 2>/dev/null; then
        PLAYWRIGHT_MODE="container"
    fi
  fi
}

wait_for_container() {
  local max_wait=30
  local elapsed=0
  echo "Waiting for container to start..."
  while ! (echo > "/dev/tcp/127.0.0.1/$PLAYWRIGHT_SERVER_PORT") 2>/dev/null; do
    if [ "$("$CONTAINER_RUNTIME" inspect -f '{{.State.Running}}' "$CONTAINER_NAME" 2>/dev/null)" != "true" ]; then
      echo "Error: container exited before becoming ready." >&2
      "$CONTAINER_RUNTIME" logs "$CONTAINER_NAME" >&2 || true
      return 1
    fi
    if [ "$elapsed" -ge "$max_wait" ]; then
      echo "Error: container did not become ready after ${max_wait}s." >&2
      "$CONTAINER_RUNTIME" logs "$CONTAINER_NAME" >&2 || true
      return 1
    fi
    sleep 1
    elapsed=$((elapsed + 1))
  done
  echo "Container is ready."
}

CMD="${1:-run}"
if [ "$CMD" = "install" ] || [ "$CMD" = "run" ]; then
  [ $# -gt 0 ] && shift
else
  CMD="run"
fi

detect_playwright_mode

if [ "$PLAYWRIGHT_MODE" = "container" ]; then
  if ! command -v "$CONTAINER_RUNTIME" >/dev/null 2>&1; then
    echo "error: PLAYWRIGHT_MODE=container but '$CONTAINER_RUNTIME' is not installed." >&2
    echo "Install docker/podman or set CONTAINER_RUNTIME to an available runtime." >&2
    exit 1
  fi
  PLAYWRIGHT_VERSION=$(sed -n 's/.*"@playwright\/test"[[:space:]]*:[[:space:]]*"[^[:digit:]]*\([^"]*\)".*/\1/p' package.json)
  if ! [[ "$PLAYWRIGHT_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]]; then
    echo "error: invalid @playwright/test version in package.json: '${PLAYWRIGHT_VERSION}'" >&2
    exit 1
  fi
  PLAYWRIGHT_IMAGE="mcr.microsoft.com/playwright:v${PLAYWRIGHT_VERSION}-noble"
fi

if [ "$CMD" = "install" ]; then
  if [ "$PLAYWRIGHT_MODE" = "local" ]; then
    # on GitHub Actions VMs, playwright's system deps are pre-installed
    if [ -z "${GITHUB_ACTIONS:-}" ]; then
      pnpm exec playwright install --with-deps chromium firefox ${PLAYWRIGHT_FLAGS:-}
    else
      pnpm exec playwright install chromium firefox ${PLAYWRIGHT_FLAGS:-}
    fi
  else
    echo "Running playwright in container as host distro is not supported by playwright directly"
    if ! "$CONTAINER_RUNTIME" image inspect "$PLAYWRIGHT_IMAGE" >/dev/null 2>&1; then
      "$CONTAINER_RUNTIME" pull "$PLAYWRIGHT_IMAGE"
    fi
  fi
  exit 0
fi

# Create isolated work directory
WORK_DIR=$(mktemp -d)

# Find a random free port
FREE_PORT=$(free_port)

cleanup() {
  if [ "$PLAYWRIGHT_MODE" = "container" ]; then
    "$CONTAINER_RUNTIME" stop "$CONTAINER_NAME" >/dev/null 2>&1 || true
  fi
  if [ -n "${SERVER_PID:-}" ]; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

if [ "$PLAYWRIGHT_MODE" = "container" ]; then
  PLAYWRIGHT_SERVER_PORT=$(free_port)
  # --network=host: container needs host loopback to reach gitea.
  "$CONTAINER_RUNTIME" run --network=host --name "$CONTAINER_NAME" -d --rm --init --workdir /home/pwuser --user pwuser "$PLAYWRIGHT_IMAGE" /bin/sh -c "npx -y playwright@${PLAYWRIGHT_VERSION} run-server --port ${PLAYWRIGHT_SERVER_PORT} --host 0.0.0.0"

  if ! wait_for_container; then
    exit 1
  fi
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

if [ "$PLAYWRIGHT_MODE" = "container" ]; then
  export PW_TEST_CONNECT_WS_ENDPOINT="ws://127.0.0.1:${PLAYWRIGHT_SERVER_PORT}/"
fi
pnpm exec playwright test "$@"
