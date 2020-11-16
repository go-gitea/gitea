# SCS: HTTP Session Management for Go

[![GoDoc](https://godoc.org/github.com/alexedwards/scs?status.png)](https://pkg.go.dev/github.com/alexedwards/scs/v2?tab=doc)
[![Build status](https://travis-ci.org/alexedwards/stack.svg?branch=master)](https://travis-ci.org/alexedwards/scs)
[![Go report card](https://goreportcard.com/badge/github.com/alexedwards/scs)](https://goreportcard.com/report/github.com/alexedwards/scs)
[![Test coverage](http://gocover.io/_badge/github.com/alexedwards/scs)](https://gocover.io/github.com/alexedwards/scs)


## Features

* Automatic loading and saving of session data via middleware.
* Choice of server-side session stores including PostgreSQL, MySQL, Redis, BadgerDB and BoltDB. Custom session stores are also supported.
* Supports multiple sessions per request, 'flash' messages, session token regeneration, and idle and absolute session timeouts.
* Easy to extend and customize. Communicate session tokens to/from clients in HTTP headers or request/response bodies.
* Efficient design. Smaller, faster and uses less memory than [gorilla/sessions](https://github.com/gorilla/sessions).

## Instructions

* [Installation](#installation)
* [Basic Use](#basic-use)
* [Configuring Session Behavior](#configuring-session-behavior)
* [Working with Session Data](#working-with-session-data)
* [Loading and Saving Sessions](#loading-and-saving-sessions)
* [Configuring the Session Store](#configuring-the-session-store)
* [Using Custom Session Stores](#using-custom-session-stores)
* [Preventing Session Fixation](#preventing-session-fixation)
* [Multiple Sessions per Request](#multiple-sessions-per-request)
* [Compatibility](#compatibility)

### Installation

This package requires Go 1.12 or newer.

```
$ go get github.com/alexedwards/scs/v2
```

Note: If you're using the traditional `GOPATH` mechanism to manage dependencies, instead of modules, you'll need to `go get` and `import` `github.com/alexedwards/scs` without the `v2` suffix.

Please use [versioned releases](https://github.com/alexedwards/scs/releases). Code in tip may contain experimental features which are subject to change.

### Basic Use

SCS implements a session management pattern following the [OWASP security guidelines](https://github.com/OWASP/CheatSheetSeries/blob/master/cheatsheets/Session_Management_Cheat_Sheet.md). Session data is stored on the server, and a randomly-generated unique session token (or *session ID*) is communicated to and from the client in a session cookie.

```go
package main

import (
	"io"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
)

var sessionManager *scs.SessionManager

func main() {
	// Initialize a new session manager and configure the session lifetime.
	sessionManager = scs.New()
	sessionManager.Lifetime = 24 * time.Hour

	mux := http.NewServeMux()
	mux.HandleFunc("/put", putHandler)
	mux.HandleFunc("/get", getHandler)

	// Wrap your handlers with the LoadAndSave() middleware.
	http.ListenAndServe(":4000", sessionManager.LoadAndSave(mux))
}

func putHandler(w http.ResponseWriter, r *http.Request) {
	// Store a new key and value in the session data.
	sessionManager.Put(r.Context(), "message", "Hello from a session!")
}

func getHandler(w http.ResponseWriter, r *http.Request) {
	// Use the GetString helper to retrieve the string value associated with a
	// key. The zero value is returned if the key does not exist.
	msg := sessionManager.GetString(r.Context(), "message")
	io.WriteString(w, msg)
}
```

```
$ curl -i --cookie-jar cj --cookie cj localhost:4000/put
HTTP/1.1 200 OK
Cache-Control: no-cache="Set-Cookie"
Set-Cookie: session=lHqcPNiQp_5diPxumzOklsSdE-MJ7zyU6kjch1Ee0UM; Path=/; Expires=Sat, 27 Apr 2019 10:28:20 GMT; Max-Age=86400; HttpOnly; SameSite=Lax
Vary: Cookie
Date: Fri, 26 Apr 2019 10:28:19 GMT
Content-Length: 0

$ curl -i --cookie-jar cj --cookie cj localhost:4000/get
HTTP/1.1 200 OK
Date: Fri, 26 Apr 2019 10:28:24 GMT
Content-Length: 21
Content-Type: text/plain; charset=utf-8

Hello from a session!
```

### Configuring Session Behavior

Session behavior can be configured via the `SessionManager` fields.

```go
sessionManager = scs.New()
sessionManager.Lifetime = 3 * time.Hour
sessionManager.IdleTimeout = 20 * time.Minute
sessionManager.Cookie.Name = "session_id"
sessionManager.Cookie.Domain = "example.com"
sessionManager.Cookie.HttpOnly = true
sessionManager.Cookie.Path = "/example/"
sessionManager.Cookie.Persist = true
sessionManager.Cookie.SameSite = http.SameSiteStrictMode
sessionManager.Cookie.Secure = true
```

Documentation for all available settings and their default values can be [found here](https://godoc.org/github.com/alexedwards/scs#SessionManager).

### Working with Session Data

Data can be set using the [`Put()`](https://godoc.org/github.com/alexedwards/scs#SessionManager.Put) method and retrieved with the [`Get()`](https://godoc.org/github.com/alexedwards/scs#SessionManager.Get) method. A variety of helper methods like [`GetString()`](https://godoc.org/github.com/alexedwards/scs#SessionManager.GetString), [`GetInt()`](https://godoc.org/github.com/alexedwards/scs#SessionManager.GetInt) and [`GetBytes()`](https://godoc.org/github.com/alexedwards/scs#SessionManager.GetBytes) are included for common data types. Please see [the documentation](https://godoc.org/github.com/alexedwards/scs#pkg-index) for a full list of helper methods.

The [`Pop()`](https://godoc.org/github.com/alexedwards/scs#SessionManager.Pop) method (and accompanying helpers for common data types) act like a one-time `Get()`, retrieving the data and removing it from the session in one step. These are useful if you want to implement 'flash' message functionality in your application, where messages are displayed to the user once only.

Some other useful functions are [`Exists()`](https://godoc.org/github.com/alexedwards/scs#SessionManager.Exists) (which returns a `bool` indicating whether or not a given key exists in the session data) and [`Keys()`](https://godoc.org/github.com/alexedwards/scs#SessionManager.Keys) (which returns a sorted slice of keys in the session data).

Individual data items can be deleted from the session using the [`Remove()`](https://godoc.org/github.com/alexedwards/scs#SessionManager.Remove) method. Alternatively, all session data can de deleted by using the [`Destroy()`](https://godoc.org/github.com/alexedwards/scs#SessionManager.Destroy) method. After calling `Destroy()`, any further operations in the same request cycle will result in a new session being created --- with a new session token and a new lifetime.

Behind the scenes SCS uses gob encoding to store session data, so if you want to store custom types in the session data they must be [registered](https://golang.org/pkg/encoding/gob/#Register) with the encoding/gob package first. Struct fields of custom types must also be exported so that they are visible to the encoding/gob package. Please [see here](https://gist.github.com/alexedwards/d6eca7136f98ec12ad606e774d3abad3) for a working example.

### Loading and Saving Sessions

Most applications will use the [`LoadAndSave()`](https://godoc.org/github.com/alexedwards/scs#SessionManager.LoadAndSave) middleware. This middleware takes care of loading and committing session data to the session store, and communicating the session token to/from the client in a cookie as necessary.

If you want to customize the behavior (like communicating the session token to/from the client in a HTTP header, or creating a distributed lock on the session token for the duration of the request) you are encouraged to create your own alternative middleware using the code in [`LoadAndSave()`](https://godoc.org/github.com/alexedwards/scs#SessionManager.LoadAndSave) as a template. An example is [given here](https://gist.github.com/alexedwards/cc6190195acfa466bf27f05aa5023f50).

Or for more fine-grained control you can load and save sessions within your individual handlers (or from anywhere in your application). [See here](https://gist.github.com/alexedwards/0570e5a59677e278e13acb8ea53a3b30) for an example.

### Configuring the Session Store

By default SCS uses an in-memory store for session data. This is convenient (no setup!) and very fast, but all session data will be lost when your application is stopped or restarted. Therefore it's useful for applications where data loss is an acceptable trade off for fast performance, or for prototyping and testing purposes. In most production applications you will want to use a persistent session store like PostgreSQL or MySQL instead.

The session stores currently included are shown in the table below. Please click the links for usage instructions and examples.

| Package                                                                               |                                                                                  |
|:------------------------------------------------------------------------------------- |----------------------------------------------------------------------------------|
| [badgerstore](https://github.com/alexedwards/scs/tree/master/badgerstore)       		| BadgerDB based session store  		                                               |
| [boltstore](https://github.com/alexedwards/scs/tree/master/boltstore)       			| BoltDB based session store  		                                               |
| [memstore](https://github.com/alexedwards/scs/tree/master/memstore)       			| In-memory session store (default)                                                |
| [mysqlstore](https://github.com/alexedwards/scs/tree/master/mysqlstore)   			| MySQL based session store                                                        |
| [postgresstore](https://github.com/alexedwards/scs/tree/master/postgresstore)         | PostgreSQL based session store                                                   |
| [redisstore](https://github.com/alexedwards/scs/tree/master/redisstore)       		| Redis based session store |
| [sqlite3store](https://github.com/alexedwards/scs/tree/master/sqlite3store) | SQLite3 based session store |

Custom session stores are also supported. Please [see here](#using-custom-session-stores) for more information.

### Using Custom Session Stores

[`scs.Store`](https://godoc.org/github.com/alexedwards/scs#Store) defines the interface for custom session stores. Any object that implements this interface can be set as the store when configuring the session.

```go
type Store interface {
	// Delete should remove the session token and corresponding data from the
	// session store. If the token does not exist then Delete should be a no-op
	// and return nil (not an error).
	Delete(token string) (err error)

	// Find should return the data for a session token from the store. If the
	// session token is not found or is expired, the found return value should
	// be false (and the err return value should be nil). Similarly, tampered
	// or malformed tokens should result in a found return value of false and a
	// nil err value. The err return value should be used for system errors only.
	Find(token string) (b []byte, found bool, err error)

	// Commit should add the session token and data to the store, with the given
	// expiry time. If the session token already exists, then the data and
	// expiry time should be overwritten.
	Commit(token string, b []byte, expiry time.Time) (err error)
}
```

### Preventing Session Fixation

To help prevent session fixation attacks you should [renew the session token after any privilege level change](https://github.com/OWASP/CheatSheetSeries/blob/master/cheatsheets/Session_Management_Cheat_Sheet.md#renew-the-session-id-after-any-privilege-level-change). Commonly, this means that the session token must to be changed when a user logs in or out of your application. You can do this using the [`RenewToken()`](https://godoc.org/github.com/alexedwards/scs#SessionManager.RenewToken) method like so:

```go
func loginHandler(w http.ResponseWriter, r *http.Request) {
	userID := 123

	// First renew the session token...
	err := sessionManager.RenewToken(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Then make the privilege-level change.
	sessionManager.Put(r.Context(), "userID", userID)
}
```

### Multiple Sessions per Request

It is possible for an application to support multiple sessions per request, with different lifetime lengths and even different stores. Please [see here for an example](https://gist.github.com/alexedwards/22535f758356bfaf96038fffad154824).

### Compatibility

This package requires Go 1.12 or newer.

It is not compatible with the [Echo](https://echo.labstack.com/) framework. Please consider using the [Echo session manager](https://echo.labstack.com/middleware/session) instead.
