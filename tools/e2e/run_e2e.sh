#!/bin/bash
set -euo pipefail

[[ -z "${GITEA_CONF}" ]] && GiteaConfig='' || GiteaConfig="-C . -c ${GITEA_CONF}"

./${GITEA_EXECUTABLE:-gitea} ${GiteaConfig} --quiet web &
npx playwright test ${E2E_TESTS:-""}

trap 'kill $(jobs -p)' EXIT
