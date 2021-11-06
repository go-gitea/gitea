# [Chi](https://github.com/go-chi/chi) proxy middleware

Forwarded headers middleware to use if application is run behind reverse proxy.

[![Documentation](https://godoc.org/github.com/chi-middleware/proxy?status.svg)](https://pkg.go.dev/github.com/chi-middleware/proxy)
[![codecov](https://codecov.io/gh/chi-middleware/proxy/branch/master/graph/badge.svg)](https://codecov.io/gh/chi-middleware/proxy)
[![Go Report Card](https://goreportcard.com/badge/github.com/chi-middleware/proxy)](https://goreportcard.com/report/github.com/chi-middleware/proxy)
[![Build Status](https://cloud.drone.io/api/badges/chi-middleware/proxy/status.svg?ref=refs/heads/master)](https://cloud.drone.io/chi-middleware/proxy)

## Usage

Import using:

```go
import "github.com/chi-middleware/proxy"
```

Use middleware with default options (trusted from proxy `127.0.0.1` and trusts only last IP address provided in header):

```go
    r := chi.NewRouter()
    r.Use(proxy.ForwardedHeaders())
```

Extend default options:

```go
    r := chi.NewRouter()
    r.Use(proxy.ForwardedHeaders(
        proxy.NewForwardedHeadersOptions().
            WithForwardLimit(2).
            ClearTrustedProxies().AddTrustedProxy("10.0.0.1"),
    ))
```

Provide custom options:

```go
    r := chi.NewRouter()
    r.Use(proxy.ForwardedHeaders(&ForwardedHeadersOptions{
        ForwardLimit: 1,
        TrustedProxies: []net.IP{
            net.IPv4(10, 0, 0, 1),
        },
    }))
```
