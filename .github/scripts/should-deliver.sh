#!/usr/bin/env bash
set -euo pipefail

message="$(jq -er '.head_commit.message // ""' "$GITHUB_EVENT_PATH")"
if [[ "$GITHUB_ACTOR" == "github-actions[bot]" || "$message" == *"[gitea-delivery]"* ]]; then
  echo "Refusing generated Gitea delivery commit." >&2
  exit 1
fi
