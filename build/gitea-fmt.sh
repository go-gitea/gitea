#!/usr/bin/env bash

set -e

GO="${GO:=go}"

# a simple self-check, make sure the current working directory is Gitea's repo
if [ ! -f ./build/gitea-fmt.sh ]; then
  echo "$0 could only run in Gitea's source directory"
  exit 1
fi

if [ "$1" != "-l" -a "$1" != "-w" ]; then
  echo "$0 could only accept '-l' (list only) or '-w' (write to files) argument"
  exit 1
fi

GO_VERSION=$(grep -Eo '^go\s+[0-9]+\.[0-9]+' go.mod | cut -d' ' -f2)

echo "Run gofumpt with Go language version $GO_VERSION ..."
gofumpt -extra -lang "$GO_VERSION" "$1" .

echo "Run codeformat ..."
$GO run ./build/codeformat.go "$1" .
