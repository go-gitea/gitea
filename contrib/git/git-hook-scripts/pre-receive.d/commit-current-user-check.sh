#!/usr/bin/env bash
#
# Pre-receive hook that will reject all pushes where author or committer are not current user.
# ref: https://github.com/github/platform-samples/blob/master/pre-receive-hooks/commit-current-user-check.sh

# If we are on Gitea web interface then we don't need to bother to validate the commit user
if [[ $SSH_ORIGINAL_COMMAND == "gitea-internal" ]]; then
  exit $retval
fi

# the return value (bit-coded error information)
retval=0

zero_commit="0000000000000000000000000000000000000000"

# Do not traverse over commits that are already in the repository
# (e.g. in a different branch)
# This prevents funny errors if pre-receive hooks got enabled after some
# commits got already in and then somebody tries to create a new branch
# If this is unwanted behavior, just set the variable to empty
excludeExisting="--not --all"

while read oldrev newrev refname; do
  # branch or tag get deleted
  if [ "$newrev" = "$zero_commit" ]; then
    continue
  fi

  # Check for new branch or tag
  if [ "$oldrev" = "$zero_commit" ]; then
    span=`git rev-list $newrev $excludeExisting`
  else
    span=`git rev-list $oldrev..$newrev $excludeExisting`
  fi

  for COMMIT in $span;
    do
      AUTHOR_USER=`git log --format=%an -n 1 ${COMMIT}`
      AUTHOR_EMAIL=`git log --format=%ae -n 1 ${COMMIT}`
      COMMIT_USER=`git log --format=%cn -n 1 ${COMMIT}`
      COMMIT_EMAIL=`git log --format=%ce -n 1 ${COMMIT}`

      if [[ ${AUTHOR_USER} != ${GITEA_PUSHER_NAME} ]]; then
        echo -e "ERROR: Commit author (${AUTHOR_USER}) does not match current user (${GITEA_PUSHER_NAME})"
        retval=$((retval + 1))
      fi

      if [[ ${COMMIT_USER} != ${GITEA_PUSHER_NAME} ]]; then
        echo -e "ERROR: Commit User (${COMMIT_USER}) does not match current user (${GITEA_PUSHER_NAME})"
        retval=$((retval + 2))
      fi

      if [[ ${AUTHOR_EMAIL} != ${GITEA_PUSHER_EMAIL} ]]; then
        echo -e "ERROR: Commit author's email (${AUTHOR_EMAIL}) does not match current user's email (${GITEA_PUSHER_EMAIL})"
        retval=$((retval + 4))
      fi

      if [[ ${COMMIT_EMAIL} != ${GITEA_PUSHER_EMAIL} ]]; then
        echo -e "ERROR: Commit user's email (${COMMIT_EMAIL}) does not match current user's email (${GITEA_PUSHER_EMAIL})"
        retval=$((retval + 8))
      fi
  done
done

exit $retval
