#!/bin/sh
exec setuser git /app/gogs/gitea web >> /var/log/gitea.log 2>&1
