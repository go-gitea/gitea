#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

BUILD_DIR="$SCRIPT_DIR/.build"
DIST_DIR="$SCRIPT_DIR/dist"
ARCHIVE_NAME="example-go-wasm.zip"

rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR" "$DIST_DIR"

export GOOS=js
export GOARCH=wasm

echo "[+] Building Go WASM binary..."
go build -o "$BUILD_DIR/plugin.wasm" ./wasm

cp manifest.json "$BUILD_DIR/"
cp render.js "$BUILD_DIR/"
cp wasm_exec.js "$BUILD_DIR/"

( cd "$BUILD_DIR" && zip -q "../dist/$ARCHIVE_NAME" manifest.json render.js wasm_exec.js plugin.wasm )

echo "[+] Wrote $DIST_DIR/$ARCHIVE_NAME"
