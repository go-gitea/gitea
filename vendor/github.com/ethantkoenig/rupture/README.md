# rupture

[![Build Status](https://travis-ci.org/ethantkoenig/rupture.svg?branch=master)](https://travis-ci.org/ethantkoenig/rupture) [![GoDoc](https://godoc.org/github.com/ethantkoenig/rupture?status.svg)](https://godoc.org/github.com/ethantkoenig/rupture) [![Go Report Card](https://goreportcard.com/badge/blevesearch/bleve)](https://goreportcard.com/report/blevesearch/bleve)

An explosive companion to the [bleve indexing library](https://www.github.com/blevesearch/bleve)

## Features

`rupture` includes the following additions to `bleve`:

- __Flushing batches__: Batches of operation which automatically flush to the underlying bleve index.
- __Sharded indices__: An index-like abstraction built on top of several underlying indices. Sharded indices provide lower write latencies for indices with large amounts of data.
- __Index metadata__: Track index version for easily managing migrations and schema changes.
