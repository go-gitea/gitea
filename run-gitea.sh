#!/bin/bash
# run-gitea.sh - Clone, build, and run Gitea locally
# Task 01 - PearlThoughts DevOps Internship
set -e

# Go must be in PATH
export PATH=$PATH:/usr/local/go/bin

# Your fork URL
GITEA_FORK="https://github.com/shubhamsingh74888/gitea.git"

# Clone (skip if already exists)
echo "[1/4] Checking Gitea repo..."
if [ -d "gitea" ]; then
  echo "      [SKIP] Gitea already cloned. Skipping..."
else
  echo "      Cloning from your fork..."
  git clone "$GITEA_FORK"
fi

cd gitea

# Frontend build
echo "[2/4] Installing frontend dependencies..."
pnpm install

echo "[3/4] Building frontend assets..."
make frontend

# Backend build
echo "[4/4] Building Gitea binary..."
TAGS="bindata" make build

echo ""
echo "================================================"
echo " Gitea built successfully!"
echo " Starting Gitea on http://localhost:3000"
echo "================================================"
./gitea web
