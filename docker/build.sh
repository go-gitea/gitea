#!/bin/sh
set -x
set -e

# Set temp environment vars
export GOPATH=/tmp/go
export PATH=${PATH}:${GOPATH}/bin
export GO15VENDOREXPERIMENT=1

# Install build deps
apk --no-cache --no-progress add --virtual build-deps build-base linux-pam-dev go

# Build Gogs
mkdir -p ${GOPATH}/src/github.com/go-gitea/
ln -s /app/gogs/ ${GOPATH}/src/github.com/go-gitea/gitea
cd ${GOPATH}/src/github.com/go-gitea/gitea
make build TAGS="sqlite cert pam"
go install

# Cleanup GOPATH & vendoring dir
rm -r $GOPATH /app/gogs/vendor

# Remove build deps
apk --no-progress del build-deps

# Create git user for Gogs
adduser -H -D -g 'Gogs Git User' git -h /data/git -s /bin/bash && passwd -u git
echo "export GOGS_CUSTOM=${GOGS_CUSTOM}" >> /etc/profile
