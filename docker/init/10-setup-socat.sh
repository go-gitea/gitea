#!/bin/bash

create_socat_links() {
    # Bind linked docker container to localhost socket using socat
    USED_PORT="3000:22"
    while read NAME ADDR PORT; do
        if test -z "$NAME$ADDR$PORT"; then
            continue
        elif echo $USED_PORT | grep -E "(^|:)$PORT($|:)" > /dev/null; then
            echo "init:socat  | Can't bind linked container ${NAME} to localhost, port ${PORT} already in use" 1>&2
        else
            SERV_FOLDER=/app/gogs/docker/s6/SOCAT_${NAME}_${PORT}
            mkdir -p ${SERV_FOLDER}
            CMD="socat -ls TCP4-LISTEN:${PORT},fork,reuseaddr TCP4:${ADDR}:${PORT}"
            echo -e "#!/bin/sh\nexec $CMD" > ${SERV_FOLDER}/run
            chmod +x ${SERV_FOLDER}/run
            USED_PORT="${USED_PORT}:${PORT}"
            echo "init:socat  | Linked container ${NAME} will be binded to localhost on port ${PORT}" 1>&2
        fi
    done << EOT
    $(env | sed -En 's|(.*)_PORT_([0-9]+)_TCP=tcp://(.*):([0-9]+)|\1 \3 \4|p')
EOT
}

# Cleanup SOCAT services and s6 event folder
# On start and on shutdown in case container has been killed
if test -f /app/gogs/docker/s6/; then
	rm -rf $(find /app/gogs/docker/s6/ -name 'event')
    rm -rf /app/gogs/docker/s6/SOCAT_*
fi

LINK=$(echo "$SOCAT_LINK" | tr '[:upper:]' '[:lower:]')
if [ "$LINK" = "false" -o "$LINK" = "0" ]; then
    echo "init:socat  | Will not try to create socat links as requested" 1>&2
else
    create_socat_links
fi
