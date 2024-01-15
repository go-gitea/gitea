#!/bin/sh

# Protect against buggy runc in docker <20.10.6 causing problems in with Alpine >= 3.14
if [ ! -x /bin/sh ]; then
  echo "Executable test for /bin/sh failed. Your Docker version is too old to run Alpine 3.14+ and Gitea. You must upgrade Docker.";
  exit 1;
fi

if [ "${USER}" != "git" ]; then
    # Rename user
    sed -i -e "s/^git\:/${USER}\:/g" /etc/passwd
fi

if [ -z "${USER_GID}" ]; then
  USER_GID="`id -g ${USER}`"
fi

if [ -z "${USER_UID}" ]; then
  USER_UID="`id -u ${USER}`"
fi

# Change GID for USER?
if [ -n "${USER_GID}" ] && [ "${USER_GID}" != "`id -g ${USER}`" ]; then
    sed -i -e "s/^${USER}:\([^:]*\):[0-9]*/${USER}:\1:${USER_GID}/" /etc/group
    sed -i -e "s/^${USER}:\([^:]*\):\([0-9]*\):[0-9]*/${USER}:\1:\2:${USER_GID}/" /etc/passwd
fi

# Change UID for USER?
if [ -n "${USER_UID}" ] && [ "${USER_UID}" != "`id -u ${USER}`" ]; then
    sed -i -e "s/^${USER}:\([^:]*\):[0-9]*:\([0-9]*\)/${USER}:\1:${USER_UID}:\2/" /etc/passwd
fi

for FOLDER in /data/gitea/conf /data/gitea/log /data/git /data/ssh; do
    mkdir -p ${FOLDER}
done

if [ $# -gt 0 ]; then
    exec "$@"
else
    exec /bin/s6-svscan /etc/s6
fi
