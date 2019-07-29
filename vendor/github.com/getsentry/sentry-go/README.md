<p align="center">
  <a href="https://sentry.io" target="_blank" align="center">
    <img src="https://sentry-brand.storage.googleapis.com/sentry-logo-black.png" width="280">
  </a>
  <br />
</p>

# Official Sentry SDK for Go

[![Build Status](https://travis-ci.com/getsentry/sentry-go.svg?branch=master)](https://travis-ci.com/getsentry/sentry-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/getsentry/sentry-go)](https://goreportcard.com/report/github.com/getsentry/sentry-go)

`sentry-go` provides a Sentry client implementation for the Go programming language. This is the next line of the Go SDK for [Sentry](https://sentry.io/), intended to replace the `raven-go` package.

> Looking for the old `raven-go` SDK documentation? See the Legacy client section [here](https://docs.sentry.io/clients/go/).
> If you want to start using sentry-go instead, check out the [migration guide](https://docs.sentry.io/platforms/go/migration/).

## Requirements

We verify this package against N-2 recent versions of Go compiler. As of June 2019, those versions are:

* 1.10
* 1.11
* 1.12

## Installation

`sentry-go` can be installed like any other Go library through `go get`:

```bash
$ go get github.com/getsentry/sentry-go
```

Or, if you are already using Go Modules, specify a version number as well:

```bash
$ go get github.com/getsentry/sentry-go@v0.1.0
```

## Configuration

To use `sentry-go`, you’ll need to import the `sentry-go` package and initialize it with the client options that will include your DSN. If you specify the `SENTRY_DSN` environment variable, you can omit this value from options and it will be picked up automatically for you. The release and environment can also be specified in the environment variables `SENTRY_RELEASE` and `SENTRY_ENVIRONMENT` respectively.

More on this in [Configuration](https://docs.sentry.io/platforms/go/config/) section.

## Usage

By default, Sentry Go SDK uses asynchronous transport, which in the code example below requires an explicit awaiting for event delivery to be finished using `sentry.Flush` method. It is necessary, because otherwise the program would not wait for the async HTTP calls to return a response, and exit the process immediately when it reached the end of the `main` function. It would not be required inside a running goroutine or if you would use `HTTPSyncTransport`, which you can read about in `Transports` section.

```go
package main

import (
    "fmt"
    "os"
    "time"

    "github.com/getsentry/sentry-go"
)

func main() {
  err := sentry.Init(sentry.ClientOptions{
    Dsn: "___DSN___",
  })

  if err != nil {
    fmt.Printf("Sentry initialization failed: %v\n", err)
  }
  
  f, err := os.Open("filename.ext")
  if err != nil {
    sentry.CaptureException(err)
    sentry.Flush(time.Second * 5)
  }
}
```

For more detailed information about how to get the most out of `sentry-go` there is additional documentation available:

- [Configuration](https://docs.sentry.io/platforms/go/config)
- [Error Reporting](https://docs.sentry.io/error-reporting/quickstart?platform=go)
- [Enriching Error Data](https://docs.sentry.io/enriching-error-data/context?platform=go)
- [Transports](https://docs.sentry.io/platforms/go/transports)
- [Integrations](https://docs.sentry.io/platforms/go/integrations)
  - [net/http](https://docs.sentry.io/platforms/go/http)
  - [echo](https://docs.sentry.io/platforms/go/echo)
  - [fasthttp](https://docs.sentry.io/platforms/go/fasthttp)
  - [gin](https://docs.sentry.io/platforms/go/gin)
  - [iris](https://docs.sentry.io/platforms/go/iris)
  - [martini](https://docs.sentry.io/platforms/go/martini)
  - [negroni](https://docs.sentry.io/platforms/go/negroni)

## Resources:

- [Bug Tracker](https://github.com/getsentry/sentry-go/issues)
- [GitHub Project](https://github.com/getsentry/sentry-go)
- [Godocs](https://godoc.org/github.com/getsentry/sentry-go)
- [@getsentry](https://twitter.com/getsentry) on Twitter for updates

## License

Licensed under the BSD license, see `LICENSE`

## Community

Want to join our Sentry's `community-golang` channel, get involved and help us improve the SDK?

Do not hesistate to shoot me up an email at [kamil@sentry.io](mailto:kamil@sentry.io) for Slack invite!
