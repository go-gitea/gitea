# openid.go

This is a consumer (Relying party) implementation of OpenId 2.0,
written in Go.

    go get -u github.com/yohcop/openid-go

[![Build Status](https://travis-ci.org/yohcop/openid-go.svg?branch=master)](https://travis-ci.org/yohcop/openid-go)

## Github

Be awesome! Feel free to clone and use according to the licence.
If you make a useful change that can benefit others, send a
pull request! This ensures that one version has all the good stuff
and doesn't fall behind.

## Code example

See `_example/` for a simple webserver using the openID
implementation. Also, read the comment about the NonceStore towards
the top of that file. The example must be run for the openid-go
directory, like so:

    go run _example/server.go

## App Engine

In order to use this on Google App Engine, you need to create an instance with a custom `*http.Client` provided by [urlfetch](https://cloud.google.com/appengine/docs/go/urlfetch/).

```go
oid := openid.NewOpenID(urlfetch.Client(appengine.NewContext(r)))
oid.RedirectURL(...)
oid.Verify(...)
```

## License

Distributed under the [Apache v2.0 license](http://www.apache.org/licenses/LICENSE-2.0.html).
