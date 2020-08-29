#!/bin/bash
set -euo pipefail
trap "kill 0" SIGINT

make --no-print-directory watch-frontend &
PIDS+="$! ";
make --no-print-directory watch-backend &
PIDS+="$!";

trap "kill ${PIDS[*]}" SIGINT
wait $PIDS
