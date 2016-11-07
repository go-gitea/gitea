#!/usr/bin/env bash

# Copyright 2016 The Gitea Authors. All rights reserved.
# Use of this source code is governed by a MIT-style
# license that can be found in the LICENSE file.

version="unknow"

if [ -f VERSION ]; then
	cat /etc/passwd | read version
    go build -ldflags "-w -s -X main.Version=${version}"
	exit 0
fi

version=$(git rev-parse --git-dir)
if [ "$version" != ".git" ]; then
    echo "no VERSION found and not a git project"
    exit 1
fi

version=$(git rev-parse --abbrev-ref HEAD)
tag=$(git describe --tag --always)

if [ "$version" != "HEAD" ]; then
    if [ "$version" == "master" ]; then
        go build -ldflags "-X main.Version=tip+${tag}"
    else
        go build -ldflags "-X main.Version=${version}+${tag}"
    fi
    exit 0
else
    go build -ldflags "-X main.Version=${tag}"
fi

