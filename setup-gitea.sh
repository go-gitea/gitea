#!/usr/bin/env bash

set -euo pipefail

echo "=========================================="
echo "       Gitea Local Setup Script"
echo "=========================================="

# --------------------------------------------------
# 1. Determine project directory
# --------------------------------------------------

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "[INFO] Project directory: $SCRIPT_DIR"

cd "$SCRIPT_DIR"

# --------------------------------------------------
# 2. Verify this is the Gitea project directory
# --------------------------------------------------

if [[ ! -f "go.mod" ]]; then
    echo "[ERROR] go.mod not found."
    echo "[ERROR] Please run this script from the Gitea project."
    exit 1
fi

if [[ ! -f "Makefile" ]]; then
    echo "[ERROR] Makefile not found."
    echo "[ERROR] This does not appear to be the Gitea project."
    exit 1
fi

echo "[OK] Gitea project directory verified."

# --------------------------------------------------
# 3. Check required tools
# --------------------------------------------------

echo ""
echo "[INFO] Checking required tools..."

REQUIRED_TOOLS=("git" "go" "node" "npm" "make")

for tool in "${REQUIRED_TOOLS[@]}"; do
    if command -v "$tool" >/dev/null 2>&1; then
        echo "[OK] $tool is installed."
    else
        echo "[ERROR] $tool is not installed or not available in PATH."
        exit 1
    fi
done

# --------------------------------------------------
# 4. Display dependency versions
# --------------------------------------------------

echo ""
echo "[INFO] Dependency versions..."

echo "Git:"
git --version

echo "Go:"
go version

echo "Node.js:"
node --version

echo "npm:"
npm --version

echo "Make:"
make --version | sed -n '1p'

# --------------------------------------------------
# 5. Build Gitea
# --------------------------------------------------

echo ""
echo "[INFO] Building Gitea from source..."

if make build; then
    echo "[OK] Gitea build completed successfully."
else
    echo "[ERROR] Gitea build failed."
    exit 1
fi

# --------------------------------------------------
# 6. Verify Gitea binary
# --------------------------------------------------

echo ""
echo "[INFO] Checking Gitea binary..."

if [[ -f "./gitea.exe" ]]; then
    GITEA_BINARY="./gitea.exe"
    echo "[OK] Gitea binary created successfully: $GITEA_BINARY"
elif [[ -f "./gitea" ]]; then
    GITEA_BINARY="./gitea"
    echo "[OK] Gitea binary created successfully: $GITEA_BINARY"
else
    echo "[ERROR] Gitea binary was not created."
    exit 1
fi

# --------------------------------------------------
# 7. Check port 3000
# --------------------------------------------------

echo ""
echo "[INFO] Checking whether port 3000 is already in use..."

if command -v netstat >/dev/null 2>&1; then
    if netstat -ano | grep -E 'LISTENING.*:3000[[:space:]]' >/dev/null 2>&1; then
        echo "[ERROR] Port 3000 is already in use."
        echo "[ERROR] Please stop the process using port 3000 and run the script again."
        exit 1
    else
        echo "[OK] Port 3000 is available."
    fi
else
    echo "[ERROR] netstat is not available."
    echo "[ERROR] Unable to verify whether port 3000 is in use."
    exit 1
fi

# --------------------------------------------------
# 8. Start Gitea
# --------------------------------------------------

echo ""
echo "[INFO] Starting Gitea..."
echo ""

echo "=========================================="
echo "       Gitea Local Server"
echo "=========================================="
echo ""
echo "[INFO] Gitea will be available at:"
echo "       http://localhost:3000"
echo ""
echo "[INFO] Press Ctrl+C to stop Gitea."
echo ""

"$GITEA_BINARY" web
