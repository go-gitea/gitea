#!/bin/sh
set -e

if [ ! -f go.mod -o ! -d snap ]; then
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

# How it works:
# * snapcraft.io checks out the default branch (e.g.: main during 1.27 dev period)
# * "override-pull" step gets the latest tag by date (e.g.: v1.26.1)
# * use "snap info gitea" to get the latest released tag
#   * if the latest tag is not released to stable, checkout that tag and build it for "stable"
#   * otherwise, build the main branch for "devel"
# * "override-build" step uses build script from the checked out commit to build
# This approach highly depends on the "main" branch's push.

# To debug the logic:
# * last_committed_tag=v1.26.1 last_released_tag=v1.26.0 ./snap/part-gitea-pull.sh
#   it will checkout tag v1.26.1, and build that for "stable"
# * last_committed_tag=v1.26.1 last_released_tag=v1.26.1 recent_tag=v1.27.0-dev-205 ./snap/part-gitea-pull.sh
#   it will still use the current branch, and build it for "devel"

[ -z "$last_committed_tag" ] && last_committed_tag="$(git for-each-ref --sort=taggerdate --format '%(tag)' refs/tags | tail -n 1)"
[ -z "$last_released_tag" ] && last_released_tag="$(snap info gitea | awk '$1 == "latest/candidate:" { print $2 }')"

if [ "${last_committed_tag}" != "${last_released_tag}" ]; then
  # if the latest tag has not been released to stable, build that tag instead of default branch.
  echo "Build last committed tag ${last_committed_tag} for new release, fetching and checking out ..."
  git fetch --quiet
  git checkout --quiet "${last_committed_tag}"
  # HINT: after this, the "build" step will use that commit's "part-gitea-build.sh", but not this one's
else
  echo "Build current branch $(git branch --show-current) @ $(git rev-parse HEAD)"
fi

# possible tag names:
# * v1.27.0-dev-205-gce089f498b
# * v1.26.1
# * v1.22.0-rc1-2816-gce089f498b
# then normalize it to semver format: v1.2.3+dev.205.gce089f498b
[ -z "$recent_tag" ] && recent_tag="$(git describe --always --tags)"
echo "Use recent tag $recent_tag to determine version and grade"

version_main="$(echo "$recent_tag" | cut -d'-' -f1)"
version_ext="$(echo "$recent_tag" | cut -d'-' -s -f2- | sed -e 'y/-/./')"
[ -n "$version_ext" ] && grade=devel || grade=stable

version="${version_main}"
[ -n "$version_ext" ] && version="${version}+${version_ext}"

craftctl set version="$version"
craftctl set grade="$grade"
