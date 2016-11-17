#!/bin/sh
set -x
set -e

# Set temp environment vars
export GOPATH=/tmp/go
export PATH=${PATH}:${GOPATH}/bin
export GO15VENDOREXPERIMENT=1

# Build Gitea
mkdir -p ${GOPATH}/src/github.com/go-gitea/
ln -s /app/gitea/ ${GOPATH}/src/github.com/go-gitea/gitea
cd ${GOPATH}/src/github.com/go-gitea/gitea

make build TAGS="sqlite cert pam"
go install

# Cleanup GOPATH & vendoring dir
rm -r $GOPATH /app/gitea/vendor

mv /app/gitea/bin/gitea /app/gitea/gitea
