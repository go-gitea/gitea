#!/bin/bash
[[ -f ./setup ]] && source ./setup

pushd /app/gitea >/dev/null
exec su-exec $USER /usr/local/bin/gitea web
popd
