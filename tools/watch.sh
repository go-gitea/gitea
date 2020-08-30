#!/bin/bash
set -euo pipefail

make watch-frontend &
make watch-backend &

trap 'kill $(jobs -p)' EXIT
wait
