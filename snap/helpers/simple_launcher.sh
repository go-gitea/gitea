#!/bin/bash

source $SNAP/bin/directorySetup.sh

# Set usernames for gitea
export USERNAME=root
export USER=root

export GITEA_WORK_DIR=$SCOMMON
export GITEA_CUSTOM=$SDATA/custom

cd $SNAP/gitea; ./gitea $@
