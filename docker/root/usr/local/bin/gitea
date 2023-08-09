#!/bin/bash

###############################################################
# This script sets defaults for gitea to run in the container #
###############################################################

# It assumes that you place this script as gitea in /usr/local/bin
#
# And place the original in /usr/lib/gitea with working files in /data/gitea
GITEA="/app/gitea/gitea"
WORK_DIR="/data/gitea"
CUSTOM_PATH="/data/gitea"

# Provide docker defaults
GITEA_WORK_DIR="${GITEA_WORK_DIR:-$WORK_DIR}" GITEA_CUSTOM="${GITEA_CUSTOM:-$CUSTOM_PATH}" exec -a "$0" "$GITEA" $CONF_ARG "$@"
