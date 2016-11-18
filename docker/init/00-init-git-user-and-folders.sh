#!/bin/bash

source /etc/profile

# Create VOLUME subfolder
for f in ${GOGS_CUSTOM}/data ${GOGS_CUSTOM}/conf ${GOGS_CUSTOM}/custom ${GOGS_CUSTOM}/log /data/git /data/ssh; do
	if ! test -d $f; then
		mkdir -p $f
	fi
done


if ! test -f ~git/.ssh/environment; then
    echo "GOGS_CUSTOM=${GOGS_CUSTOM}" > ~git/.ssh/environment
    chmod 660 ~git/.ssh/environment
fi

# Link volumed data with app data
ln -sf ${GOGS_CUSTOM}/log  /app/gitea/log
ln -sf ${GOGS_CUSTOM}/data /app/gitea/data
ln -sf ${GOGS_CUSTOM}/custom /app/gitea/custom

# Backward Compatibility with Gogs Container v0.6.15
ln -sf /data/git /home/git

chmod 0775 ${GOGS_CUSTOM} ${GOGS_CUSTOM}/custom ${GOGS_CUSTOM}/log ${GOGS_CUSTOM}/data
