#!/usr/bin/env bash

set -u

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

log() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

fail() {
    error "$1"
    exit 1
}

echo
echo "=========================================="
echo "      Gitea Local Setup Automation"
echo "=========================================="
echo

# --------------------------------------------------
# 1. Determine project directory
# --------------------------------------------------

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

log "Checking project directory..."
cd "$SCRIPT_DIR" || fail "Unable to enter script directory."

if [[ ! -f "go.mod" || ! -f "Makefile" || ! -f "main.go" ]]; then
    fail "This script must be located in the Gitea project root."
fi

success "Gitea project directory verified:"
echo "       $SCRIPT_DIR"

# --------------------------------------------------
# 2. Check required tools
# --------------------------------------------------

log "Checking required development tools..."

REQUIRED_TOOLS=(
    git
    go
    node
    npm
    make
    pnpm
    uv
)

for tool in "${REQUIRED_TOOLS[@]}"; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        fail "Required tool not found: $tool"
    fi

    success "$tool is installed"
done

# --------------------------------------------------
# 3. Display dependency versions
# --------------------------------------------------

echo
log "Installed tool versions:"
echo

echo "Git:"
git --version

echo
echo "Go:"
go version

echo
echo "Node.js:"
node --version

echo
echo "npm:"
npm --version

echo
echo "Make:"
make --version | head -n 1

echo
echo "pnpm:"
pnpm --version

echo
echo "uv:"
uv --version

# --------------------------------------------------
# 4. Verify Go version required by project
# --------------------------------------------------

echo
log "Checking Go version required by Gitea..."

REQUIRED_GO_VERSION="$(grep '^go ' go.mod | awk '{print $2}')"

if [[ -z "$REQUIRED_GO_VERSION" ]]; then
    fail "Could not determine required Go version from go.mod."
fi

echo "Required Go version: $REQUIRED_GO_VERSION"

# --------------------------------------------------
# 5. Build Gitea from source
# --------------------------------------------------

echo
log "Starting Gitea source build..."
echo

if ! make build; then
    fail "Gitea build failed."
fi

success "Gitea build completed successfully."

# --------------------------------------------------
# 6. Verify Gitea binary
# --------------------------------------------------

echo
log "Checking Gitea binary..."

if [[ ! -f "gitea" && ! -f "gitea.exe" ]]; then
    fail "Gitea binary was not created."
fi

if [[ -f "gitea" ]]; then
    GITEA_BINARY="./gitea"
else
    GITEA_BINARY="./gitea.exe"
fi

success "Gitea binary found: $GITEA_BINARY"

# --------------------------------------------------
# 7. Check port 3000
# --------------------------------------------------

echo
log "Checking whether port 3000 is already in use..."

PORT_CHECK="$(netstat -ano 2>/dev/null | grep -E '[:.]3000[[:space:]]' || true)"

if [[ -n "$PORT_CHECK" ]]; then
    warning "Port 3000 is already in use."
    echo
    echo "$PORT_CHECK"
    echo
    fail "Cannot start Gitea because port 3000 is already in use."
fi

success "Port 3000 is available."

# --------------------------------------------------
# 8. Start Gitea
# --------------------------------------------------

echo
log "Starting Gitea web server..."

LOG_FILE="$SCRIPT_DIR/gitea-task-2.log"

"$GITEA_BINARY" web > "$LOG_FILE" 2>&1 &
GITEA_PID=$!

sleep 5

if ! kill -0 "$GITEA_PID" 2>/dev/null; then
    error "Gitea failed to start."
    echo
    echo "Last log output:"
    tail -30 "$LOG_FILE"
    exit 1
fi

success "Gitea web server started."
echo
echo "=========================================="
echo " Gitea is running successfully"
echo "=========================================="
echo
echo "Local URL:"
echo "http://localhost:3000"
echo
echo "Process ID: $GITEA_PID"
echo "Log file: $LOG_FILE"
echo
echo "Open http://localhost:3000 in your browser."
echo