#!/bin/sh

CROND=$(echo "$RUN_CROND" | tr '[:upper:]' '[:lower:]')
if [ "$CROND" = "true" -o "$CROND" = "1" ]; then
    echo "init:crond  | Cron Daemon (crond) will be run as requested by s6" 1>&2
    rm -f /app/gogs/docker/s6/crond/down
else
    # Tell s6 not to run the crond service
    touch /app/gogs/docker/s6/crond/down
fi

# Exec CMD or S6 by default if nothing present
if [ $# -gt 0 ];then
    exec "$@"
else
    exec /bin/s6-svscan /app/gogs/docker/s6/
fi
