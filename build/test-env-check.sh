#!/bin/sh

set -e

cd -- "$(dirname -- "${BASH_SOURCE[0]}")"/.. # cd into parent folder

echo "check uid ..."

# the uid of gitea defined in "https://gitea.com/gitea/test-env" is 1000
gitea_uid=$(id -u gitea)
if [ "$gitea_uid" != "1000" ]; then
  echo "The uid of linux user 'gitea' is expected to be 1000, but it is $gitea_uid"
  exit 1
fi

cur_uid=$(id -u)
if [ "$cur_uid" != "0" -a "$cur_uid" != "$gitea_uid" ]; then
  echo "The uid of current linux user is expected to be 0 or $gitea_uid, but it is $cur_uid"
  exit 1
fi
