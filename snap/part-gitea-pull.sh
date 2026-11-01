#!/bin/sh
set -e

if [ ! -f go.mod ] || [ ! -d snap ]; then
  echo "This script should be run from the root of the gitea repository"
  exit 1
fi

if [ -z "$SNAPCRAFT_PROJECT_NAME" ]; then
  echo "No snapcraft build env, continue with mocking ...."
  craftctl() { echo "* mocking craftctl: $*"; }
  snap() { echo "* mocking snap: $*"; }
else
  echo "Add working directory to git safe.directory ..."
  git config --global --add safe.directory "$PWD"
fi

# Build the newest tag for "stable" when "snap info gitea" shows it unreleased in "latest/candidate", else the current branch for "devel".
# Debug with e.g. last_committed_tag=v1.26.1 last_released_tag=v1.26.0 recent_tag=v1.27.0-dev-205 ./snap/part-gitea-pull.sh

[ -z "$last_committed_tag" ] && last_committed_tag="$(git for-each-ref --sort=taggerdate --format '%(tag)' refs/tags | tail -n 1)"
[ -z "$last_released_tag" ] && last_released_tag="$(snap info gitea | awk '$1 == "latest/candidate:" { print $2 }')"

if [ "${last_committed_tag}" != "${last_released_tag}" ]; then
  echo "Build last committed tag ${last_committed_tag} for new release, fetching and checking out ..."
  git fetch --quiet
  git checkout --quiet "${last_committed_tag}"
  # the build step then runs the tag's part-gitea-build.sh, not this one
else
  echo "Build current branch $(git branch --show-current) @ $(git rev-parse HEAD)"
fi

# normalize e.g. v1.27.0-dev-205-gce089f498b to semver v1.27.0+dev.205.gce089f498b
[ -z "$recent_tag" ] && recent_tag="$(git describe --always --tags)"
echo "Use recent tag $recent_tag to determine version and grade"

version_main="$(echo "$recent_tag" | cut -d'-' -f1)"
version_ext="$(echo "$recent_tag" | cut -d'-' -s -f2- | sed -e 'y/-/./')"
[ -n "$version_ext" ] && grade=devel || grade=stable

version="${version_main}"
[ -n "$version_ext" ] && version="${version}+${version_ext}"

craftctl set version="$version"
craftctl set grade="$grade"
