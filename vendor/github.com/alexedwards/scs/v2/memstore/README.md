# memstore

An in-memory session store for [SCS](https://github.com/alexedwards/scs).


Because memstore uses in-memory storage only, all session data will be lost when your application is stopped or restarted. Therefore it should only be used in applications where data loss is an acceptable trade off for fast performance, or for prototyping and testing purposes.

## Example

```go
package main

import (
	"io"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
)

var sessionManager *scs.SessionManager

func main() {
	// Initialize a new session manager and configure it to use memstore as
	// the session store.
	sessionManager = scs.New()
	sessionManager.Store = memstore.New()

	mux := http.NewServeMux()
	mux.HandleFunc("/put", putHandler)
	mux.HandleFunc("/get", getHandler)

	http.ListenAndServe(":4000", sessionManager.LoadAndSave(mux))
}

func putHandler(w http.ResponseWriter, r *http.Request) {
	sessionManager.Put(r.Context(), "message", "Hello from a session!")
}

func getHandler(w http.ResponseWriter, r *http.Request) {
	msg := sessionManager.GetString(r.Context(), "message")
	io.WriteString(w, msg)
}
```

## Expired Session Cleanup

This package provides a background 'cleanup' goroutine to delete expired session data. This stops the database table from holding on to invalid sessions indefinitely and growing unnecessarily large. By default the cleanup runs once every minute. You can change this by using the `NewWithCleanupInterval()` function to initialize your session store. For example:

```go
// Run a cleanup every 30 seconds.
memstore.NewWithCleanupInterval(db, 30*time.Second)

// Disable the cleanup goroutine by setting the cleanup interval to zero.
memstore.NewWithCleanupInterval(db, 0)
```

### Terminating the Cleanup Goroutine

It's rare that the cleanup goroutine needs to be terminated --- it is generally intended to be long-lived and run for the lifetime of your application.

However, there may be occasions when your use of a session store instance is transient. A common example would be using it in a short-lived test function. In this scenario, the cleanup goroutine (which will run forever) will prevent the session store instance from being garbage collected even after the test function has finished. You can prevent this by either disabling the cleanup goroutine altogether (as described above) or by stopping it using the `StopCleanup()` method. For example:

```go
func TestExample(t *testing.T) {
	store := memstore.New()
	defer store.StopCleanup()

	sessionManager = scs.New()
	sessionManager.Store = store

	// Run test...
}
```