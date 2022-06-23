#!/bin/bash
set -euo pipefail

# Kill any processes on exit
trap 'kill $(jobs -p)' EXIT

GiteaFlags=()

[[ -v GITEA_CUSTOM ]] && GiteaFlags+=(-C "${GITEA_CUSTOM}")
[[ -v GITEA_CONF ]] && GiteaFlags+=(-c "${GITEA_CONF}")

./"${GITEA_EXECUTABLE:-gitea}" "${GiteaFlags[@]}" --quiet web &

# Wait up to 30s for server to start
timeout 30 bash -c 'while [[ "$(curl -s -o /dev/null -w ''%{http_code}'' ${GITEA_URL:-http://localhost:3000})" != "200" ]]; do sleep 2; done' || \
  (echo -e "\033[0;31mTimed out testing server up: ${GITEA_URL:-http://localhost:3000}\033[0m"; false)

npx playwright test ${E2E_TESTS:-""}
