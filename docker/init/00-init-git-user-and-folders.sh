#!/bin/bash
# Create VOLUME subfolder
for f in /data/gitea/data /data/gitea/conf /data/gitea/custom /data/gitea/log /data/git /data/ssh; do
	if ! test -d $f; then
		mkdir -p $f
	fi
done

export GOGS_CUSTOM=/data/gitea

if ! test -f ~git/.ssh/environment; then
    echo "GOGS_CUSTOM=${GOGS_CUSTOM}" > ~git/.ssh/environment
    chmod 660 ~git/.ssh/environment
fi

# Link volumed data with app data
ln -sf /data/gitea/log  ./log
ln -sf /data/gitea/data ./data
ln -sf /data/gitea/custom ./custom

# Backward Compatibility with Gogs Container v0.6.15
ln -sf /data/git /home/git

chown -R git:root /data /app/gitea
chmod 0775 /data /data/gitea /data/gitea/custom /data/gitea/log /data/gitea/data /app/gitea

echo "export GOGS_CUSTOM=${GOGS_CUSTOM}" >> /etc/profile
