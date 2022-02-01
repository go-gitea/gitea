#!/bin/bash
set -euo pipefail

./${GITEA_EXECUTABLE:-gitea} web --quiet &
npx playwright test ${E2E_TESTS:-""}

trap 'kill $(jobs -p)' EXIT
