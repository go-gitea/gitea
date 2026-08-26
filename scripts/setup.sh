#!/bin/bash

set -euo pipefail

echo "=========================================="
echo "      Gitea Local Setup Automation"
echo "=========================================="

# --------------------------------------------------
# 1. Verify project directory
# --------------------------------------------------

echo ""
echo "[1/7] Checking project directory..."

if [ ! -f "go.mod" ] || [ ! -f "Makefile" ]; then
    echo "ERROR: This script must be run from the Gitea project root."
    echo "Please run:"
    echo "cd /path/to/gitea"
    exit 1
fi

echo "Project directory verified:"
pwd


# --------------------------------------------------
# 2. Check required tools
# --------------------------------------------------

echo ""
echo "[2/7] Checking required tools..."

check_command() {
    if command -v "$1" >/dev/null 2>&1; then
        echo "✓ $1: installed"
    else
        echo "ERROR: $1 is not installed."
        exit 1
    fi
}

check_command git
check_command go
check_command node
check_command pnpm
check_command make

echo ""
echo "Dependency versions:"
echo "Git:  $(git --version)"
echo "Go:   $(go version)"
echo "Node: $(node -v)"
echo "pnpm: $(pnpm -v)"
echo "Make: $(make --version | head -n 1)"


# --------------------------------------------------
# 3. Build Gitea
# --------------------------------------------------

echo ""
echo "[3/7] Building Gitea..."

if make build; then
    echo "✓ Gitea build completed successfully."
else
    echo "ERROR: Gitea build failed."
    exit 1
fi


# --------------------------------------------------
# 4. Verify Gitea binary
# --------------------------------------------------

echo ""
echo "[4/7] Checking Gitea binary..."

if [ -f "./gitea" ] && [ -x "./gitea" ]; then
    echo "✓ Gitea binary created successfully."
    ls -lh ./gitea
else
    echo "ERROR: Gitea binary was not created."
    exit 1
fi


# --------------------------------------------------
# 5. Check port 3000
# --------------------------------------------------

echo ""
echo "[5/7] Checking port 3000..."

if lsof -iTCP:3000 -sTCP:LISTEN >/dev/null 2>&1; then
    echo "ERROR: Port 3000 is already in use."
    echo ""
    echo "Process using port 3000:"
    lsof -iTCP:3000 -sTCP:LISTEN
    exit 1
else
    echo "✓ Port 3000 is available."
fi


# --------------------------------------------------
# 6. Display local URL
# --------------------------------------------------

echo ""
echo "[6/7] Gitea will be available at:"
echo "http://localhost:3000"


# --------------------------------------------------
# 7. Start Gitea
# --------------------------------------------------

echo ""
echo "[7/7] Starting Gitea..."
echo ""
echo "=========================================="
echo " Gitea server starting..."
echo " URL: http://localhost:3000"
echo "=========================================="
echo ""
