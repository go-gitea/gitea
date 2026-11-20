#!/bin/bash
set -euo pipefail

# Prints the revert commits in the given range and the commits they revert, for git-cliff's .cliffignore
range="${1:?usage: $0 <revision-range>}"
commits=$(git log --format='%H %s' "$range")

shopt -s nocasematch
while read -r sha subject; do
  [[ "$subject" == revert* ]] || continue
  echo "$sha"
  for pr in $(git show -s --format=%B "$sha" | grep -i revert | grep -oE '(#|/pull/)[0-9]+' | grep -oE '[0-9]+' || true); do
    grep -F "(#$pr)" <<< "$commits" | cut -d' ' -f1 || true
  done
done <<< "$commits"
