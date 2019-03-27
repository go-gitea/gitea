#!/bin/sh

patch < sig-v3.patch
patch < s2k-gnu-dummy.patch
find . -type f -name '*.go' -exec sed -i'' -e 's/golang.org\/x\/crypto\/openpgp/github.com\/keybase\/go-crypto\/openpgp/' {} \;
find . -type f -name '*.go-e' -exec rm {} \;
go test ./...
