# Wire: Automated Initialization in Go

[![Build Status](https://travis-ci.com/google/wire.svg?branch=master)][travis]
[![godoc](https://godoc.org/github.com/google/wire?status.svg)][godoc]
[![Coverage](https://codecov.io/gh/google/wire/branch/master/graph/badge.svg)](https://codecov.io/gh/google/wire)


Wire is a code generation tool that automates connecting components using
[dependency injection][]. Dependencies between components are represented in
Wire as function parameters, encouraging explicit initialization instead of
global variables. Because Wire operates without runtime state or reflection,
code written to be used with Wire is useful even for hand-written
initialization.

For an overview, see the [introductory blog post][].

[dependency injection]: https://en.wikipedia.org/wiki/Dependency_injection
[introductory blog post]: https://blog.golang.org/wire
[godoc]: https://godoc.org/github.com/google/wire
[travis]: https://travis-ci.com/google/wire

## Installing

Install Wire by running:

```shell
go get github.com/google/wire/cmd/wire
```

and ensuring that `$GOPATH/bin` is added to your `$PATH`.

## Documentation

- [Tutorial][]
- [User Guide][]
- [Best Practices][]
- [FAQ][]

[Tutorial]: ./_tutorial/README.md
[Best Practices]: ./docs/best-practices.md
[FAQ]: ./docs/faq.md
[User Guide]: ./docs/guide.md

## Project status

**This project is in alpha and is not yet suitable for production.**

While in alpha, the API is subject to breaking changes.

## Community

You can contact us on the [go-cloud mailing list][].

This project is covered by the Go [Code of Conduct][].

[Code of Conduct]: ./CODE_OF_CONDUCT.md
[go-cloud mailing list]: https://groups.google.com/forum/#!forum/go-cloud
