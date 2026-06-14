#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME="gitea"
SERVICE_USER="git"
SERVICE_GROUP="git"
SERVICE_HOME="/home/${SERVICE_USER}"
CONFIG_DIR="/etc/${SERVICE_NAME}"
CONFIG_FILE="${CONFIG_DIR}/app.ini"
WORK_DIR="/var/lib/${SERVICE_NAME}"
INSTALL_BIN="/usr/local/bin/${SERVICE_NAME}"
BINARY_NAME="${SERVICE_NAME}"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

if [[ "${EUID}" -ne 0 ]]; then
  echo "请使用 root 权限运行: sudo $0" >&2
  exit 1
fi

# 脚本位于 contrib/，工作目录取仓库根
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_DIR}"

echo "拉取最新代码..."
git pull

echo "编译 Gitea (前端 + 后端)..."
make clean-all >/dev/null 2>&1 || true
TAGS="${TAGS:-bindata sqlite sqlite_unlock_notify}" make build

if [[ ! -x "${REPO_DIR}/${BINARY_NAME}" ]]; then
  echo "构建产物 ${REPO_DIR}/${BINARY_NAME} 不存在，构建失败" >&2
  exit 1
fi

echo "检查系统用户..."
if ! getent group "${SERVICE_GROUP}" >/dev/null 2>&1; then
  groupadd --system "${SERVICE_GROUP}"
fi
if ! id -u "${SERVICE_USER}" >/dev/null 2>&1; then
  useradd --system --gid "${SERVICE_GROUP}" \
    --create-home --home-dir "${SERVICE_HOME}" \
    --shell /bin/bash \
    --comment 'Git Version Control' \
    "${SERVICE_USER}"
fi

echo "创建目录..."
install -d -m 0750 -o "${SERVICE_USER}" -g "${SERVICE_GROUP}" "${CONFIG_DIR}"
install -d -m 0750 -o "${SERVICE_USER}" -g "${SERVICE_GROUP}" \
  "${WORK_DIR}" \
  "${WORK_DIR}/custom" \
  "${WORK_DIR}/data" \
  "${WORK_DIR}/log" \
  "${WORK_DIR}/repositories"

echo "安装二进制..."
# 停服后再覆盖二进制，避免 ETXTBSY
systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
install -m 0755 -o root -g root "${REPO_DIR}/${BINARY_NAME}" "${INSTALL_BIN}"

if [[ ! -f "${CONFIG_FILE}" ]]; then
  echo "写入初始 app.ini (首次运行可在 web 安装界面继续配置)..."
  cat > "${CONFIG_FILE}" <<EOF
APP_NAME = Gitea: Git with a cup of tea
RUN_USER = ${SERVICE_USER}
RUN_MODE = prod

[server]
PROTOCOL  = http
HTTP_ADDR = 0.0.0.0
HTTP_PORT = 3000
DOMAIN    = localhost
ROOT_URL  = http://localhost:3000/
DISABLE_SSH = false
SSH_PORT  = 22
START_SSH_SERVER = false
APP_DATA_PATH = ${WORK_DIR}/data

[repository]
ROOT = ${WORK_DIR}/repositories

[database]
DB_TYPE  = sqlite3
PATH     = ${WORK_DIR}/data/gitea.db

[log]
ROOT_PATH = ${WORK_DIR}/log
MODE      = console,file
LEVEL     = Info

[security]
INSTALL_LOCK = false

[service]
DISABLE_REGISTRATION = false
REQUIRE_SIGNIN_VIEW  = false

[session]
PROVIDER = file

[picture]
DISABLE_GRAVATAR = false

[oauth2]
ENABLED = true
EOF
  chown "${SERVICE_USER}:${SERVICE_GROUP}" "${CONFIG_FILE}"
  chmod 0640 "${CONFIG_FILE}"
fi

echo "写入 systemd 服务文件..."
cat > "${SERVICE_FILE}" <<EOF
[Unit]
Description=Gitea (Git with a cup of tea)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_GROUP}
WorkingDirectory=${WORK_DIR}
RuntimeDirectory=${SERVICE_NAME}
ExecStart=${INSTALL_BIN} web --config ${CONFIG_FILE}
Restart=always
RestartSec=2s
Environment=USER=${SERVICE_USER} HOME=${SERVICE_HOME} GITEA_WORK_DIR=${WORK_DIR}
# 允许绑定 1024 以下端口（如 80/443）；不需要时可去掉
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_BIND_SERVICE
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ReadWritePaths=${CONFIG_DIR} ${WORK_DIR}

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable "${SERVICE_NAME}"
systemctl restart "${SERVICE_NAME}"

echo "部署完成，服务状态："
systemctl --no-pager --full status "${SERVICE_NAME}"
