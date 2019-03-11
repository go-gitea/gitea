#!/bin/bash -e

# CONTRIBUTING.md asks contributors to run this script before
# submitting PRs.  Please update the next several lines if drone-cli
# version or cmdline needs to change.

drone_cmdline="drone exec --local --build-event pull_request"

drone_url_amd64="https://github.com/drone/drone-cli/releases/download/v0.8.6/drone_linux_amd64.tar.gz"
drone_md5_amd64="9103bcee8dee9932fbb49aef7906449f"

# drone containers need a lot of space -- 20Gb should be enough for now
space_avail_min=20000000

# check platform
case $OSTYPE in
  linux*)
    if uname -a | grep Linux | grep x86_64
    then
      drone_url=$drone_url_amd64
      drone_md5=$drone_md5_amd64
      space_avail=$(df -k --output=avail . | tail -1)
    fi
    ;;
  # insert other platforms here
esac

# confirm platform
if [ -z "$drone_url" ] || [ -z "$drone_md5" ]
then
  echo
  echo This script is not supported on this os/hardware -- see
  echo CONTRIBUTING.md for manual execution instructions.
  exit 1
fi

# confirm we're in repo directory
if ! [ -e .drone.yml ]
then
  echo "no .drone.yml found -- are you in gitea repository root?"
fi
repodir=$PWD

# check disk space -- #6243
if [ "$space_avail" -lt "$space_avail_min" ]
then
  echo not enough disk space: wanted ${space_avail_min}k, found ${space_avail}k
  exit 1
fi

set -x

# create working directory
tmpdir=/tmp/gitea-drone.$$
mkdir $tmpdir
cd $tmpdir

# fetch drone-cli
curl -L $drone_url > drone.tar.gz
md5=$(md5sum drone.tar.gz | awk '{print $1}')
if [ "$md5" != "$drone_md5" ]
then
  echo md5 mismatch: wanted $drone_md5, got $md5
  exit 1
fi

# unpack into ./drone
tar -xf drone.tar.gz  
# ensure exists and is executable 
chmod +x ./drone 

# run tests
cd $repodir
$tmpdir/$drone_cmdline

# show some stats
echo
df -k .
uptime

# clean up
cd /tmp
rm -rf $tmpdir

