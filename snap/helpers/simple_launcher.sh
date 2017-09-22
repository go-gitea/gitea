#!/bin/bash

if ! env | grep -q root; then
  echo "
   +----------------------------------------+
   | You are not running gitea as root.     |
   | This is required for the snap package. |
   | Please re-run as root.                 |
   +----------------------------------------+
"
  $SNAP/gitea/gitea --help
  exit 1
fi

# Set usernames for gitea
export USERNAME=root
export USER=root

export GITEA_WORK_DIR=$(snapctl get GITEA_WORK_DIR)
export GITEA_CUSTOM=$(snapctl get GITEA_CUSTOM)

cd $SNAP/gitea; ./gitea $@
