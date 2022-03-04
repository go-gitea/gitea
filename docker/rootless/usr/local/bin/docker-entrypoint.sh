#!/bin/sh

# Protect against buggy runc in docker <20.10.6 causing problems in with Alpine >= 3.14
if [ ! -x /bin/sh ]; then
  echo "Executable test for /bin/sh failed. Your Docker version is too old to run Alpine 3.14+ and Gitea. You must upgrade Docker.";
  exit 1;
fi

if [ -x /usr/local/bin/docker-setup.sh ]; then
    /usr/local/bin/docker-setup.sh || { echo 'docker setup failed' ; exit 1; }
fi

if [ $# -gt 0 ]; then
    exec "$@"
else
    exec /usr/local/bin/gitea -c ${GITEA_APP_INI} web
fi
