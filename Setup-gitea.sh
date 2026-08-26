#!/bin/bash
# Setup-gitea.sh - Build and run Gitea locally without manual steps
# Task 02 - PearlThoughts DevOps Internship
set -e

# Go must be in PATH
export PATH=$PATH:/usr/local/go/bin

# Reusable separator constant
SEPARATOR="================================================"

echo "${SEPARATOR}"
echo " Gitea Automated Setup Script                   "
echo " Task 02 - PearlThoughts DevOps Internship      "
echo "${SEPARATOR}"
echo ""

# -----------------------------------------------------------------------------
# STEP 1: Verify script is running from the correct project directory
# -----------------------------------------------------------------------------
echo "[1/6] Verifying project directory..."

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GITEA_DIR="${SCRIPT_DIR}/gitea"

echo "      Script location : ${SCRIPT_DIR}"
echo "      Gitea source    : ${GITEA_DIR}"

if [[ ! -d "${GITEA_DIR}" ]]; then
    echo "[ERROR] Gitea source folder not found at: ${GITEA_DIR}" >&2
    echo "        Copy the gitea folder from task-01 into this directory." >&2
    exit 1
fi

if [[ ! -f "${GITEA_DIR}/go.mod" ]] || [[ ! -f "${GITEA_DIR}/Makefile" ]]; then
    echo "[ERROR] ${GITEA_DIR} does not look like a Gitea source folder." >&2
    echo "        Expected: go.mod and Makefile to be present." >&2
    exit 1
fi

echo "      Directory check passed."
echo ""

# -----------------------------------------------------------------------------
# STEP 2: Check required tools are installed and display versions
# -----------------------------------------------------------------------------
echo "[2/6] Checking required tools and dependency versions..."

MISSING=0

if command -v go &>/dev/null; then
    echo "      Go    : $(go version)"
else
    echo "      [MISSING] go is not installed" >&2
    MISSING=1
fi

if command -v git &>/dev/null; then
    echo "      Git   : $(git --version)"
else
    echo "      [MISSING] git is not installed" >&2
    MISSING=1
fi

if command -v make &>/dev/null; then
    echo "      Make  : $(make --version | head -1)"
else
    echo "      [MISSING] make is not installed" >&2
    MISSING=1
fi

if command -v node &>/dev/null; then
    echo "      Node  : $(node --version)"
else
    echo "      [MISSING] node is not installed" >&2
    MISSING=1
fi

if command -v pnpm &>/dev/null; then
    echo "      pnpm  : $(pnpm --version)"
else
    echo "      [MISSING] pnpm is not installed" >&2
    MISSING=1
fi

if [[ "${MISSING}" = "1" ]]; then
    echo "" >&2
    echo "[ERROR] One or more required tools are missing." >&2
    echo "        Run setup.sh from task-01 to install all dependencies." >&2
    exit 1
fi

echo "      All tools are present."
echo ""

# -----------------------------------------------------------------------------
# STEP 3: Check whether port 3000 is already in use
# -----------------------------------------------------------------------------
echo "[3/6] Checking port 3000..."

if lsof -i :3000 &>/dev/null 2>&1; then
    echo "      [WARN] Port 3000 is already in use."
    echo "      This is likely Gitea still running from Task 01."
    echo ""
    echo "      Process using port 3000:"
    lsof -i :3000 | grep LISTEN || true
    echo ""
    echo "      Killing the process..."
    kill "$(lsof -t -i:3000)" 2>/dev/null || true
    sleep 2

    if lsof -i :3000 &>/dev/null 2>&1; then
        echo "[ERROR] Port 3000 is still in use." >&2
        echo "        Run manually: sudo kill \$(lsof -t -i:3000)" >&2
        exit 1
    fi

    echo "      Port 3000 is now free."
else
    echo "      Port 3000 is free."
fi
echo ""

# -----------------------------------------------------------------------------
# STEP 4: Build Gitea from source
# -----------------------------------------------------------------------------
echo "[4/6] Building Gitea from source..."
echo ""

cd "${GITEA_DIR}"
echo "      Working directory: $(pwd)"
echo ""

# Install frontend dependencies
echo "      Installing frontend dependencies (pnpm install)..."
pnpm install --ignore-scripts
echo "      Frontend dependencies installed."
echo ""

# Build frontend assets (JS + CSS)
echo "      Building frontend assets (make frontend)..."
make frontend
echo "      Frontend build complete."
echo ""

# Build backend binary
echo "      Building backend binary (TAGS=bindata make backend)..."
echo "      This may take a few minutes — please wait..."
echo ""
TAGS="bindata" make backend
echo ""
echo "      Backend build complete."
echo ""

# -----------------------------------------------------------------------------
# STEP 5: Verify that the Gitea binary was created successfully
# -----------------------------------------------------------------------------
echo "[5/6] Verifying Gitea binary..."

BINARY="${GITEA_DIR}/gitea"

if [[ ! -f "${BINARY}" ]]; then
    echo "[ERROR] Binary not found at: ${BINARY}" >&2
    echo "        The build may have failed. Check output above." >&2
    exit 1
fi

if [[ ! -x "${BINARY}" ]]; then
    echo "[ERROR] Binary exists but is not executable: ${BINARY}" >&2
    exit 1
fi

echo "      Binary found    : ${BINARY}"
echo "      Gitea version   : $("${BINARY}" --version 2>&1 | head -n1)"
echo "      Binary check passed."
echo ""

# -----------------------------------------------------------------------------
# STEP 6: Start the Gitea web server
# -----------------------------------------------------------------------------
echo "[6/6] Starting Gitea web server..."
echo ""
echo "${SEPARATOR}"
echo " Gitea built successfully!"
echo " Starting Gitea on http://localhost:3000"
echo "${SEPARATOR}"
echo ""

"${BINARY}" web
