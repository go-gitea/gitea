#!/usr/bin/env bash

grep 'git' go.mod | grep '\.com' | grep -v indirect | grep -v replace | cut -f 2 | cut -d ' ' -f 1 | while read line; do
  go get -u "$line"
  git add .
  git commit -m "update $line"
done

make vendor
