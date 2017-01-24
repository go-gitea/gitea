#!/bin/bash -e

version=$(git describe --tags --always | sed 's/-/+/' | sed 's/^v//')
echo Version:$version
go test -ldflags "-X code.gitea.io/gitea/tests.Version=$version" $@
