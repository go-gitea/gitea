# `raven-go` to `sentry-go` Migration Guide

## Installation

raven-go

```go
go get github.com/getsentry/raven-go
```

sentry-go

```go
go get github.com/getsentry/sentry-go@v0.0.1
```

## Configuration

raven-go

```go
import "github.com/getsentry/raven-go"

func main() {
    raven.SetDSN("https://16427b2f210046b585ee51fd8a1ac54f@sentry.io/1")
}
```

sentry-go

```go
import (
    "fmt"
    "github.com/getsentry/sentry-go"
)

func main() {
    err := sentry.Init(sentry.ClientOptions{
        Dsn: "https://16427b2f210046b585ee51fd8a1ac54f@sentry.io/1",
    })

    if err != nil {
        fmt.Printf("Sentry initialization failed: %v\n", err)
    }
}
```

raven-go

```go
SetDSN()
SetDefaultLoggerName()
SetDebug()
SetEnvironment()
SetRelease()
SetSampleRate()
SetIgnoreErrors()
SetIncludePaths()
```

sentry-go

```go
sentry.Init(sentry.ClientOptions{
    Dsn: "https://16427b2f210046b585ee51fd8a1ac54f@sentry.io/1",
    DebugWriter: os.Stderr,
    Debug: true,
    Environment: "environment",
    Release: "release",
    SampleRate: 0.5,
    // IgnoreErrors: TBD,
    // IncludePaths: TBD
})
```

Available options: see [Configuration](https://docs.sentry.io/platforms/go/config/) section.

### Providing SSL Certificates

By default, TLS uses the host's root CA set. If you don't have `ca-certificates` (which should be your go-to way of fixing the issue of missing ceritificates) and want to use `gocertifi` instead, you can provide pre-loaded cert files as one of the options to the `sentry.Init` call:

```go
package main

import (
    "log"

    "github.com/certifi/gocertifi"
    "github.com/getsentry/sentry-go"
)

sentryClientOptions := sentry.ClientOptions{
    Dsn: "https://16427b2f210046b585ee51fd8a1ac54f@sentry.io/1",
}

rootCAs, err := gocertifi.CACerts()
if err != nil {
    log.Println("Coudnt load CA Certificates: %v\n", err)
} else {
    sentryClientOptions.CaCerts = rootCAs
}

sentry.Init(sentryClientOptions)
```

## Usage

### Capturing Errors

raven-go

```go
f, err := os.Open("filename.ext")
if err != nil {
    raven.CaptureError(err, nil)
}
```

sentry-go

```go
f, err := os.Open("filename.ext")
if err != nil {
    sentry.CaptureException(err)
}
```

### Capturing Panics

raven-go

```go
raven.CapturePanic(func() {
    // do all of the scary things here
}, nil)
```

sentry-go

```go
func() {
    defer sentry.Recover()
    // do all of the scary things here
}()
```

### Capturing Messages

raven-go

```go
raven.CaptureMessage("Something bad happened and I would like to know about that")
```

sentry-go

```go
sentry.CaptureMessage("Something bad happened and I would like to know about that")
```

### Capturing Events

raven-go

```go
packet := &raven.Packet{
    Message: "Hand-crafted event",
    Extra: &raven.Extra{
        "runtime.Version": runtime.Version(),
        "runtime.NumCPU": runtime.NumCPU(),
    },
}
raven.Capture(packet)
```

sentry-go

```go
event := &sentry.NewEvent()
event.Message = "Hand-crafted event"
event.Extra["runtime.Version"] = runtime.Version()
event.Extra["runtime.NumCPU"] = runtime.NumCPU()

sentry.CaptureEvent(event)
```

### Additional Data

See Context section.

### Event Sampling

raven-go

```go
raven.SetSampleRate(0.25)
```

sentry-go

```go
sentry.Init(sentry.ClientOptions{
    SampleRate: 0.25,
})
```

### Awaiting the response (not recommended)

```go
raven.CaptureMessageAndWait("Something bad happened and I would like to know about that")
```

sentry-go

```go
sentry.CaptureMessage("Something bad happened and I would like to know about that")

if sentry.Flush(time.Second * 2) {
    // event delivered
} else {
    // timeout reached
}
```

## Context

### Per-event

raven-go

```go
raven.CaptureError(err, map[string]string{"browser": "Firefox"}, &raven.Http{
    Method: "GET",
    URL: "https://example.com/raven-go"
})
```

sentry-go

```go
sentry.WithScope(func(scope *sentry.Scope) {
    scope.SetTag("browser", "Firefox")
    scope.SetContext("Request", map[string]string{
        "Method": "GET",
        "URL": "https://example.com/raven-go",
    })
    sentry.CaptureException(err)
})
```

### Globally

#### SetHttpContext

raven-go

```go
raven.SetHttpContext(&raven.Http{
    Method: "GET",
    URL: "https://example.com/raven-go",
})
```

sentry-go

```go
sentry.ConfigureScope(func(scope *sentry.Scope) {
    scope.SetContext("Request", map[string]string{
        "Method": "GET",
        "URL": "https://example.com/raven-go",
    })
})
```

#### SetTagsContext

raven-go

```go
t := map[string]string{"day": "Friday", "sport": "Weightlifting"}
raven.SetTagsContext(map[string]string{"day": "Friday", "sport": "Weightlifting"})
```

sentry-go

```go
sentry.ConfigureScope(func(scope *sentry.Scope) {
    scope.SetTags(map[string]string{"day": "Friday", "sport": "Weightlifting"})
})
```

#### SetUserContext

raven-go

```go
raven.SetUserContext(&raven.User{
    ID: "1337",
    Username: "kamilogorek",
    Email: "kamil@sentry.io",
    IP: "127.0.0.1",
})
```

sentry-go

```go
sentry.ConfigureScope(func(scope *sentry.Scope) {
    scope.SetUser(sentry.User{
        ID: "1337",
        Username: "kamilogorek",
        Email: "kamil@sentry.io",
        IPAddress: "127.0.0.1",
    })
})
```

#### ClearContext

raven-go

```go
raven.ClearContext()
```

sentry-go

```go
sentry.ConfigureScope(func(scope *sentry.Scope) {
    scope.Clear()
})
```

#### WrapWithExtra

raven-go

```go
path := "filename.ext"
f, err := os.Open(path)
if err != nil {
    err = raven.WrapWithExtra(err, map[string]string{"path": path, "cwd": os.Getwd()}
    raven.CaptureError(err, nil)
}
```

sentry-go

```go
// use `sentry.WithScope`, see "Context / Per-event Section"
path := "filename.ext"
f, err := os.Open(path)
if err != nil {
    sentry.WithScope(func(scope *sentry.Scope) {
        sentry.SetExtras(map[string]interface{}{"path": path, "cwd": os.Getwd())
        sentry.CaptureException(err)
    })
}
```

## Integrations

### net/http

raven-go

```go
mux := http.NewServeMux
http.Handle("/", raven.Recoverer(mux))

// or

func root(w http.ResponseWriter, r *http.Request) {}
http.HandleFunc("/", raven.RecoveryHandler(root))
```

sentry-go

```go
sentryHandler := sentryhttp.New(sentryhttp.Options{
    Repanic: false,
    WaitForDelivery: true,
})

mux := http.NewServeMux
http.Handle("/", sentryHandler.Handle(mux))

// or

func root(w http.ResponseWriter, r *http.Request) {}
http.HandleFunc("/", sentryHandler.HandleFunc(root))
```
