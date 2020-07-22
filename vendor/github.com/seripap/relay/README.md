# GraphQL Relay Go [![Build Status](https://travis-ci.com/seripap/relay.svg?branch=master)](https://travis-ci.com/seripap/relay) [![GoDoc](https://godoc.org/graphql-go/relay?status.svg)](https://godoc.org/github.com/seripap/relay) [![Coverage Status](https://coveralls.io/repos/github/seripap/relay/badge.svg?branch=master)](https://coveralls.io/github/seripap/relay?branch=master)

A Go/Golang library to help construct a [graphql-go](https://github.com/graphql-go/graphql) server supporting react-relay. This is a fork and updated version of [graphql-go-relay](https://github.com/graphql-go/relay).

### Notes:
- Using Context provided from standard lib
- Added `totalCount` and `nodes` to Connection Type
- `clientMutationId` is an optional input paramater

### Test
```bash
$ go get github.com/seripap/relay
$ go build && go test ./...
```
