#!/bin/bash

mkdir -p /etc/service/99-gitea

cp /app/gitea/docker/gitea.sh /etc/service/99-gitea/run

chmod -R 775 /etc/service/99-gitea
chown -R git:root /etc/service/99-gitea

export GOGS_CUSTOM=/data/gitea
echo "export GOGS_CUSTOM=${GOGS_CUSTOM}" >> /etc/profile

chown -R git:root /data /app/gitea
chmod 0775 /data /app/gitea
cp /app/gitea/docker/init/00-init-git-user-and-folders.sh /etc/my_init.d/99-gitea
