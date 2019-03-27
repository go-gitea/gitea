[![GoDoc](https://godoc.org/github.com/lafriks/xormstore?status.svg)](https://godoc.org/github.com/lafriks/xormstore)
[![Build Status](https://travis-ci.org/lafriks/xormstore.svg?branch=master)](https://travis-ci.org/lafriks/xormstore)
[![codecov](https://codecov.io/gh/lafriks/xormstore/branch/master/graph/badge.svg)](https://codecov.io/gh/lafriks/xormstore)

#### XORM backend for gorilla sessions

    go get github.com/lafriks/xormstore

#### Example

```go
// initialize and setup cleanup
store := xormstore.New(engine, []byte("secret"))
// db cleanup every hour
// close quit channel to stop cleanup
quit := make(chan struct{})
go store.PeriodicCleanup(1*time.Hour, quit)
```

```go
// in HTTP handler
func handlerFunc(w http.ResponseWriter, r *http.Request) {
  session, err := store.Get(r, "session")
  session.Values["user_id"] = 123
  store.Save(r, w, session)
  http.Error(w, "", http.StatusOK)
}
```

For more details see [xormstore godoc documentation](https://godoc.org/github.com/lafriks/xormstore).

#### Testing

Just sqlite3 tests:

    go test

All databases using docker:

    ./test

If docker is not local (docker-machine etc):

    DOCKER_IP=$(docker-machine ip dev) ./test

#### License

xormstore is licensed under the MIT license. See [LICENSE](LICENSE) for the full license text.
