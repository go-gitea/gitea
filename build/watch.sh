#!/bin/bash
set -euo pipefail

make --no-print-directory watch-frontend &
make --no-print-directory watch-backend &

trap 'kill $(jobs -p)' EXIT
wait
