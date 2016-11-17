#!/usr/bin/env bash
apt-get update
DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends software-properties-common
add-apt-repository -y ppa:ubuntu-lxc/lxd-stable
apt-get update
DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends build-essential libpam-runtime libpam0g-dev golang
