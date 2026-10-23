#!/bin/bash

# Prepare git folder
mkdir -p "${HOME}" && chmod 0700 "${HOME}"
if [ ! -w "${HOME}" ]; then echo "${HOME} is not writable"; exit 1; fi

# Prepare custom folder
mkdir -p "${CODENEST_CUSTOM}" && chmod 0700 "${CODENEST_CUSTOM}"

# Prepare temp folder
mkdir -p "${CODENEST_TEMP}" && chmod 0700 "${CODENEST_TEMP}"
if [ ! -w "${CODENEST_TEMP}" ]; then echo "${CODENEST_TEMP} is not writable"; exit 1; fi

#Prepare config file
if [ ! -f "${CODENEST_APP_INI}" ]; then

    #Prepare config file folder
    CODENEST_APP_INI_DIR=$(dirname "${CODENEST_APP_INI}")
    mkdir -p "${CODENEST_APP_INI_DIR}" && chmod 0700 "${CODENEST_APP_INI_DIR}"
    if [ ! -w "${CODENEST_APP_INI_DIR}" ]; then echo "${CODENEST_APP_INI_DIR} is not writable"; exit 1; fi

    # Set INSTALL_LOCK to true only if SECRET_KEY is not empty and
    # INSTALL_LOCK is empty
    if [ -n "$SECRET_KEY" ] && [ -z "$INSTALL_LOCK" ]; then
        INSTALL_LOCK=true
    fi

    # Substitute the environment variables in the template
    APP_NAME=${APP_NAME:-"CodeNest: Git with a cup of tea"} \
    RUN_MODE=${RUN_MODE:-"prod"} \
    RUN_USER=${USER:-"git"} \
    SSH_DOMAIN=${SSH_DOMAIN:-"localhost"} \
    HTTP_PORT=${HTTP_PORT:-"3000"} \
    ROOT_URL=${ROOT_URL:-""} \
    DISABLE_SSH=${DISABLE_SSH:-"false"} \
    SSH_PORT=${SSH_PORT:-"2222"} \
    SSH_LISTEN_PORT=${SSH_LISTEN_PORT:-} \
    DB_TYPE=${DB_TYPE:-"sqlite3"} \
    DB_HOST=${DB_HOST:-"localhost:3306"} \
    DB_NAME=${DB_NAME:-"codenest"} \
    DB_USER=${DB_USER:-"root"} \
    DB_PASSWD=${DB_PASSWD:-""} \
    INSTALL_LOCK=${INSTALL_LOCK:-"false"} \
    DISABLE_REGISTRATION=${DISABLE_REGISTRATION:-"false"} \
    REQUIRE_SIGNIN_VIEW=${REQUIRE_SIGNIN_VIEW:-"false"} \
    SECRET_KEY=${SECRET_KEY:-""} \
    envsubst < /etc/templates/app.ini > "${CODENEST_APP_INI}"
fi

# Replace app.ini settings with env variables in the form CODENEST__SECTION_NAME__KEY_NAME
environment-to-ini --config "${CODENEST_APP_INI}"
