#!/bin/bash

mkdir -p /etc/service/99-gitea

cp /app/gitea/docker/gitea.sh /etc/service/99-gitea/run

chmod -R 775 /etc/service/99-gitea
chown -R git:root /etc/service/99-gitea
