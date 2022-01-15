#!/bin/bash
set -e

# this is an upgrade script, for gitea deployed on linux as systemd service
# depends on: curl, xz, sha256sum, gpg
# usage:      upgrade.sh [version]

# change the variables below for your local setup
giteaversion=${1:-1.15.10}
giteabin=/usr/local/bin/gitea
giteahome=/var/lib/gitea
giteaconf=/etc/gitea/app.ini
giteauser="git"
giteacmd="sudo -u $giteauser $giteabin -c $giteaconf -w $giteahome"

# download new binary
binname=gitea-${giteaversion}-linux-amd64
binurl="https://dl.gitea.io/gitea/${giteaversion}/${binname}.xz"
echo downloading $binurl
cd $giteahome # needed for gitea dump later
curl -sSfL "$binurl" > ${binname}.xz
curl -sSfL "${binurl}.sha256" > ${binname}.xz.sha256
curl -sSfL "${binurl}.asc" > ${binname}.xz.asc

# validate checksum & gpg signature (exit script if error)
sha256sum -c ${binname}.xz.sha256
gpg --keyserver keys.openpgp.org --recv 7C9E68152594688862D62AF62D9AE806EC1592E2
gpg --verify ${binname}.xz.asc ${binname}.xz
rm ${binname}.xz.{sha256,asc}

# unpack binary + make executable
xz -d ${binname}.xz
chmod +x $binname

# stop gitea, create backup, replace binary, restart gitea
$giteacmd manager flush-queues
systemctl stop gitea
$giteacmd --version
$giteacmd dump
mv -fb $binname $giteabin
systemctl start gitea
