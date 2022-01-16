#!/usr/bin/env bash
set -e

# This is an update script for gitea deployed from the binary distribution
# from dl.gitea.io on linux as systemd service. It performs backup and updates
# Gitea in place.
# Depends on: bash, curl, xz, sha256sum, gpg, which. optionally jq.
# Usage:      [environment vars] upgrade.sh [version]
#   See below section for available environment vars.
#   When no version is specied, updates to the latest release.
# Examples:
#   upgrade.sh 1.15.10
#   giteahome=/opt/gitea giteaconf=$giteahome/app.ini upgrade.sh

# apply variables from environment
: ${giteabin:=/usr/local/bin/gitea}
: ${giteahome:=/var/lib/gitea}
: ${giteaconf:=/etc/gitea/app.ini}
: ${giteauser:=git}
: ${sudocmd:=sudo}
: ${arch:=linux-amd64}

function giteacmd {
  "$sudocmd" -u "$giteauser" "$giteabin" -c "$giteaconf" -w "$giteahome" $@
}

function require {
  for exe in $@; do
    which $exe &>/dev/null || (echo "missing dependency '$exe'"; exit 1)
  done
}
require curl xz sha256sum gpg

# select version to install
if [[ -z "$1" ]]; then
  require jq
	giteaversion=`curl -sL https://dl.gitea.io/gitea/version.json | jq -r .latest.version`
else
	giteaversion="${1}"
fi

# confirm update
current=`giteacmd --version | cut -d' ' -f3`
echo "make sure to read the changelog first: https://github.com/go-gitea/gitea/blob/main/CHANGELOG.md"
echo "are you ready to update Gitea from ${current} to ${giteaversion}? (y/N)"
read confirm
[[ "$confirm" == "y" ]] || exit 1

pushd `pwd`
cd $giteahome # needed for gitea dump later

# download new binary
binname=gitea-${giteaversion}-${arch}
binurl="https://dl.gitea.io/gitea/${giteaversion}/${binname}.xz"
echo "Downloading $binurl..."
curl -sSfLO "$binurl{,.sha256,.asc}"

# validate checksum & gpg signature (exit script if error)
sha256sum -c ${binname}.xz.sha256
# TODO 2022-06-24: this gpg key will expire!
gpg --keyserver keys.openpgp.org --recv 7C9E68152594688862D62AF62D9AE806EC1592E2
gpg --verify ${binname}.xz.asc ${binname}.xz
rm ${binname}.xz.{sha256,asc}

# unpack binary + make executable
xz -d ${binname}.xz
chmod +x $binname

# stop gitea, create backup, replace binary, restart gitea
giteacmd manager flush-queues
$sudocmd systemctl stop gitea
giteacmd dump
mv -fb $binname $giteabin
$sudocmd systemctl start gitea

popd
