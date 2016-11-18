#!/bin/bash

source /etc/profile

# Create VOLUME subfolder
for f in ${GITEA_CUSTOM}/data ${GITEA_CUSTOM}/conf ${GITEA_CUSTOM}/custom ${GITEA_CUSTOM}/log /data/git /data/ssh; do
	if ! test -d $f; then
		mkdir -p $f
	fi
done


if ! test -f ~git/.ssh/environment; then
    echo "GITEA_CUSTOM=${GITEA_CUSTOM}" > ~git/.ssh/environment
    chmod 660 ~git/.ssh/environment
fi

# Link volumed data with app data
ln -sf ${GITEA_CUSTOM}/log  /app/gitea/log
ln -sf ${GITEA_CUSTOM}/data /app/gitea/data
ln -sf ${GITEA_CUSTOM}/custom /app/gitea/custom

# Backward Compatibility with Gogs Container v0.6.15
ln -sf /data/git /home/git

chmod 0775 ${GITEA_CUSTOM} ${GITEA_CUSTOM}/custom ${GITEA_CUSTOM}/log ${GITEA_CUSTOM}/data
