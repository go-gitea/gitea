#!/bin/bash
set -euo pipefail
trap "kill 0" SIGINT SIGTERM SIGQUIT

make watch-frontend &
PIDS+="$! ";
make watch-backend &
PIDS+="$!";

trap "kill ${PIDS[*]}" SIGINT SIGTERM SIGQUIT
wait $PIDS
