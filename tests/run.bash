#!/bin/bash -e

go test -ldflags "-X code.gitea.io/gitea/tests.Version=$(git describe --tags --always | sed 's/-/+/' | sed 's/^v//')" $@
