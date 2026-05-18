#!/bin/sh
set -e

if [ ! -f go.mod -o ! -d snap ]; then
  echo "This script should be run from the root of the gitea repository"
  exit 1
fi

if [ -z "$SNAPCRAFT_PART_INSTALL" ]; then
  SNAPCRAFT_PART_INSTALL="./dist/snap"
  echo "* using mock install path: $SNAPCRAFT_PART_INSTALL"
fi

command -v pnpm >/dev/null 2>&1 || npm install -g pnpm

# build tags for 1.26: "bindata sqlite sqlite_unlock_notify pam cert"
# for 1.27: sqlite is not needed, "cert" seems doing nothing
TAGS="bindata pam" make build

install -D gitea "${SNAPCRAFT_PART_INSTALL}/gitea"
cp -r options "${SNAPCRAFT_PART_INSTALL}/"
