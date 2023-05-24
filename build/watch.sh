#!/bin/bash
set -euo pipefail

make watch-frontend &
make watch-backend &

trap 'kill -9 $(jobs -p)' EXIT
wait
