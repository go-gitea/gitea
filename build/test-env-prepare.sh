#!/bin/sh

set -e

if [ ! -f ./build/test-env-prepare.sh ]; then
  echo "${0} can only be executed in gitea source root directory"
  exit 1
fi

echo "change the owner of files to gitea ..."
chown -R gitea:gitea .
