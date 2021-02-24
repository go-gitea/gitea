#!/usr/bin/env bash
set -ex

# This script builds archiver for most common platforms.

export CGO_ENABLED=0

cd cmd/arc
GOOS=linux   GOARCH=amd64 go build -o ../../builds/arc_linux_amd64
GOOS=linux   GOARCH=arm   go build -o ../../builds/arc_linux_arm7
GOOS=darwin  GOARCH=amd64 go build -o ../../builds/arc_mac_amd64
GOOS=windows GOARCH=amd64 go build -o ../../builds/arc_windows_amd64.exe
cd ../..
