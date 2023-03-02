#!/bin/sh

set -e

cd -- "$(dirname -- "${BASH_SOURCE[0]}")"/.. # cd into parent folder

echo "change the owner of files to gitea ..."
chown -R gitea:gitea .
