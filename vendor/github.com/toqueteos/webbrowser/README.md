# webbrowser [![Build Status](https://travis-ci.org/toqueteos/webbrowser.png?branch=master)](https://travis-ci.org/toqueteos/webbrowser) [![GoDoc](http://godoc.org/github.com/toqueteos/webbrowser?status.png)](http://godoc.org/github.com/toqueteos/webbrowser) [![Sourcegraph](https://sourcegraph.com/github.com/toqueteos/webbrowser/-/badge.svg)](https://sourcegraph.com/github.com/toqueteos/webbrowser?badge)

webbrowser provides a simple API for opening web pages on your default browser.

It's inspired on [Python's webbrowser](http://docs.python.org/3/library/webbrowser.html) package but lacks some of its features (open new window).

It just opens a webpage, most browsers will open it on a new tab.

## Installation

As simple as:

```bash
go get -u github.com/toqueteos/webbrowser
```

## Usage

```go
package main

import "github.com/toqueteos/webbrowser"

func main() {
    webbrowser.Open("http://golang.org")
}
```

That's it!

## Crossplatform support

The package works on:

- [x] `android` (verified by 3rd party)
- [x] `darwin`
- [x] `freebsd` (verified by 3rd party)
- [x] `linux`
- [x] `netbsd` (verified by 3rd party)
- [x] `openbsd` (verified by 3rd party)
- [x] `windows`

## License

It is licensed under the MIT open source license, please see the [LICENSE.md](https://github.com/toqueteos/webbrowser/blob/master/LICENSE.md) file for more information.

## Thanks...

Miki Tebeka wrote a nicer version that wasn't on godoc.org when I did this, [check it out!](https://bitbucket.org/tebeka/go-wise/src/d8db9bf5c4d1/desktop.go?at=default).

## Already disliking it?

No problem! There's alternative libraries that may be better to your needs:

- https://github.com/pkg/browser, it does what webbrowser does and more!
- https://github.com/skratchdot/open-golang, it even provides a `xdg-open` implementation in case you don't have it!
