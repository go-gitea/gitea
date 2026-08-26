#!/usr/bin/env bash
set -e
echo "======================================"
echo "     Gitea Local Setup Automation"
echo "======================================"

# --------------------------------------------------
# 1. Verify project directory
# --------------------------------------------------
echo ""
echo "[1/8] Checking Gitea project directory..."
if [ ! -f "go.mod" ] || [ ! -f "Makefile" ]; then
    echo "ERROR: This script must be executed from the Gitea project root."
    echo "Please run the script from the directory containing go.mod and Makefile."
    exit 1
fi
echo "Project directory verified."

# --------------------------------------------------
# 2. Check required tools
# --------------------------------------------------
echo ""
echo "[2/8] Checking required tools..."
REQUIRED_TOOLS=("go" "node" "npm" "make" "git")
for TOOL in "${REQUIRED_TOOLS[@]}"; do
    if ! command -v "$TOOL" >/dev/null 2>&1; then
        echo "ERROR: $TOOL is not installed or not available in PATH."
        exit 1
    fi
done
echo "All required tools are available."

# --------------------------------------------------
# 3. Display dependency versions
# --------------------------------------------------
echo ""
echo "[3/8] Dependency versions..."
echo "Go:"
go version
echo "Node.js:"
node --version
echo "npm:"
npm --version
echo "Make:"
make --version | head -n 1
echo "Git:"
git --version

# --------------------------------------------------
# 4. Install frontend dependencies
# --------------------------------------------------
echo ""
echo "[4/8] Installing frontend dependencies (npm install)..."
echo "Note: npm is used instead of pnpm here — pnpm's symlinked"
echo "node_modules store can hit permission/module-resolution"
echo "errors on Windows (e.g. ERR_MODULE_NOT_FOUND for rolldown)."
echo "npm avoids this by installing a flat, non-symlinked tree."
if ! npm install; then
    echo "ERROR: npm install failed."
    exit 1
fi
echo "Frontend dependencies installed."

# --------------------------------------------------
# 5. Build Gitea
# --------------------------------------------------
echo ""
echo "[5/8] Building Gitea..."
if ! make build; then
    echo "ERROR: Gitea build failed."
    exit 1
fi
echo "Gitea build completed successfully."

# --------------------------------------------------
# 6. Verify Gitea binary
# --------------------------------------------------
echo ""
echo "[6/8] Checking Gitea binary..."
GITEA_BINARY="./gitea"
if [ ! -f "$GITEA_BINARY" ]; then
    GITEA_BINARY="./gitea.exe"
fi
if [ ! -f "$GITEA_BINARY" ]; then
    echo "ERROR: Gitea binary was not created."
    exit 1
fi
echo "Gitea binary found: $GITEA_BINARY"

# --------------------------------------------------
# 7. Check port 3000
# --------------------------------------------------
echo ""
echo "[7/8] Checking port 3000..."
if command -v netstat >/dev/null 2>&1; then
    if netstat -ano | grep -E '[:.]3000[[:space:]]' >/dev/null 2>&1; then
        echo "ERROR: Port 3000 is already in use."
        echo "Please stop the application using port 3000 and run the script again."
        exit 1
    fi
else
    echo "WARNING: Could not check port 3000."
fi
echo "Port 3000 is available."

# --------------------------------------------------
# 8. Start Gitea
# --------------------------------------------------
echo ""
echo "[8/8] Starting Gitea..."
echo ""
echo "Gitea will be available at:"
echo "http://localhost:3000"
echo ""
echo "Press Ctrl+C to stop Gitea."
echo ""
"$GITEA_BINARY" web
