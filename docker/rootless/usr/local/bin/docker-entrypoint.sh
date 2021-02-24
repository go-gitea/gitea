#!/bin/sh

docker_switch_user() {
    GITEA_USERNAME=${GITEA_USERNAME:-"git"}
    GITEA_GROUPNAME=${GITEA_GROUPNAME:-"git"}
    GITEA_UID=${GITEA_UID:-"1000"}
    GITEA_GID=${GITEA_GID:-"1000"}
    GITEA_HOME=${HOME:-"/var/lib/gitea/git"}

    addgroup -S -g "${GITEA_GID}" "${GITEA_GROUPNAME}" && \
    adduser -S -H -D -h "${HOME}" -s /bin/bash \
            -u "${GITEA_UID}" -G "${GITEA_GROUPNAME}" "${GITEA_USERNAME}" 
    chown -R "${GITEA_USERNAME}:${GITEA_GROUPNAME}" "${GITEA_CUSTOM}" "${GITEA_WORK_DIR}" "${GITEA_TEMP}" /etc/gitea
    echo "${GITEA_USERNAME}:$(dd if=/dev/urandom bs=24 count=1 status=none | base64)" | chpasswd

    if [ $# -gt 0 ]; then
        exec su-exec "${GITEA_USERNAME}:${GITEA_GROUPNAME}" "$@" 
    else
        exec su-exec "${GITEA_USERNAME}:${GITEA_GROUPNAME}" /usr/local/bin/gitea -c ${GITEA_APP_INI} web
    fi
}

USER=${GITEA_USERNAME:-"git"}

if [ -x /usr/local/bin/docker-setup.sh ]; then
    /usr/local/bin/docker-setup.sh || { echo 'docker setup failed' ; exit 1; }
fi

docker_switch_user
