#!/usr/bin/env bash
set -euo pipefail

# Rebuilds runtime config on each start so rsync --delete deployments are safe.
# Usage:
#   INTERNAL_TOKEN=... ./contrib/agent-enrollment/start-instance.sh
#   INTERNAL_TOKEN_FILE=~/.config/gitea-agent/internal_token ./contrib/agent-enrollment/start-instance.sh

REPO_DIR="${REPO_DIR:-$PWD}"
GITEA_BIN="${GITEA_BIN:-$REPO_DIR/gitea}"
WORK_DIR="${WORK_DIR:-$HOME/gitea-agent}"
APP_INI="$WORK_DIR/custom/conf/app.ini"

DOMAIN="${DOMAIN:-repo.scalytics.io}"
HTTP_ADDR="${HTTP_ADDR:-127.0.0.1}"
HTTP_PORT="${HTTP_PORT:-3000}"
SSH_PORT="${SSH_PORT:-2222}"
DB_PATH="${DB_PATH:-$WORK_DIR/data/gitea.db}"
NO_REPLY_ADDRESS="${NO_REPLY_ADDRESS:-noreply.repo.scalytics.io}"
INSTALL_LOCK="${INSTALL_LOCK:-true}"

INTERNAL_TOKEN="${INTERNAL_TOKEN:-}"
INTERNAL_TOKEN_FILE="${INTERNAL_TOKEN_FILE:-}"

if [[ -z "$INTERNAL_TOKEN" && -n "$INTERNAL_TOKEN_FILE" ]]; then
  if [[ ! -f "$INTERNAL_TOKEN_FILE" ]]; then
    echo "INTERNAL_TOKEN_FILE not found: $INTERNAL_TOKEN_FILE" >&2
    exit 1
  fi
  INTERNAL_TOKEN="$(head -n 1 "$INTERNAL_TOKEN_FILE" | tr -d '\r\n')"
fi

if [[ -z "$INTERNAL_TOKEN" ]]; then
  echo "INTERNAL_TOKEN is required" >&2
  exit 1
fi

mkdir -p "$WORK_DIR/custom/conf" "$WORK_DIR/data" "$WORK_DIR/log"
mkdir -p "$WORK_DIR/public"

# Keep runtime static assets in sync with the built repository assets.
if [[ -d "$REPO_DIR/public/assets" ]]; then
  rsync -a --delete "$REPO_DIR/public/assets/" "$WORK_DIR/public/assets/"
fi

cat >"$APP_INI" <<EOF
[server]
APP_DATA_PATH = $WORK_DIR/data
DOMAIN = $DOMAIN
HTTP_PORT = $HTTP_PORT
HTTP_ADDR = $HTTP_ADDR
ROOT_URL = https://$DOMAIN/
LANDING_PAGE = login
SSH_DOMAIN = $DOMAIN
SSH_PORT = $SSH_PORT
START_SSH_SERVER = true
LFS_START_SERVER = true

[database]
DB_TYPE = sqlite3
PATH = $DB_PATH

[service]
DISABLE_REGISTRATION = true
NO_REPLY_ADDRESS = $NO_REPLY_ADDRESS

[security]
INSTALL_LOCK = $INSTALL_LOCK
INTERNAL_TOKEN = $INTERNAL_TOKEN

[log]
ROOT_PATH = $WORK_DIR/log
EOF

echo "Starting gitea with config: $APP_INI"
exec "$GITEA_BIN" web --work-path "$WORK_DIR" --config "$APP_INI"
