#!/bin/bash
# setup.sh - Install all dependencies to run Gitea locally
# Task 01 - PearlThoughts DevOps Internship
set -e

echo "================================================"
echo " Gitea Local Setup - Dependency Installation"
echo "================================================"

# Step 1: System packages
echo "[1/5] Installing system packages..."
sudo apt-get update -y
sudo apt-get install -y git make gcc build-essential wget curl

# Step 2: Go 1.22
echo "[2/5] Installing Go 1.22..."
if command -v go &>/dev/null; then
  echo "      [SKIP] Go already installed: $(go version)"
else
  wget -q https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
  sudo rm -rf /usr/local/go
  sudo tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
  rm go1.22.0.linux-amd64.tar.gz
  echo "      Go installed successfully"
fi

# Add Go to PATH permanently
if ! grep -q '/usr/local/go/bin' ~/.bashrc; then
  echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
fi
export PATH=$PATH:/usr/local/go/bin
go version

# Step 3: Node.js 22
echo "[3/5] Installing Node.js 22..."
if command -v node &>/dev/null; then
  echo "      [SKIP] Node.js already installed: $(node --version)"
else
  curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
  sudo apt-get install -y nodejs
  echo "      Node.js installed successfully"
fi
node --version

# Step 4: pnpm
echo "[4/5] Installing pnpm..."
if command -v pnpm &>/dev/null; then
  echo "      [SKIP] pnpm already installed: $(pnpm --version)"
else
  sudo npm install -g pnpm
  echo "      pnpm installed successfully"
fi
pnpm --version

# Step 5: Verify everything
echo "[5/5] Verifying all installations..."
echo "      Go:   $(go version)"
echo "      Node: $(node --version)"
echo "      pnpm: $(pnpm --version)"
echo "      Git:  $(git --version)"
echo "      Make: $(make --version | head -1)"

echo ""
echo "================================================"
echo " All dependencies installed successfully!"
echo " Now run: bash run-gitea.sh"
echo "================================================"
