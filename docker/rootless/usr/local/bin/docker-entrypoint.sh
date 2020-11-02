#!/bin/sh

if [ -x /usr/local/bin/docker-setup.sh ]; then
    /usr/local/bin/docker-setup.sh || { echo 'docker setup failed' ; exit 1; }
fi

if [ $# -gt 0 ]; then
    exec "$@"
else
    exec /usr/local/bin/gitea -c ${GITEA_APP_INI} web
fi
