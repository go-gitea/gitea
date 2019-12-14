[![Build Status](https://travis-ci.org/msteinert/pam.svg?branch=master)](https://travis-ci.org/msteinert/pam)
[![GoDoc](https://godoc.org/github.com/msteinert/pam?status.svg)](http://godoc.org/github.com/msteinert/pam)
[![Coverage Status](https://coveralls.io/repos/msteinert/pam/badge.svg?branch=master)](https://coveralls.io/r/msteinert/pam?branch=master)
[![Go Report Card](http://goreportcard.com/badge/msteinert/pam)](http://goreportcard.com/report/msteinert/pam)

# Go PAM

This is a Go wrapper for the PAM application API.

## Testing

To run the full suite, the tests must be run as the root user. To setup your
system for testing, create a user named "test" with the password "secret". For
example:

```
$ sudo useradd test \
    -d /tmp/test \
    -p '$1$Qd8H95T5$RYSZQeoFbEB.gS19zS99A0' \
    -s /bin/false
```

Then execute the tests:

```
$ sudo GOPATH=$GOPATH $(which go) test -v
```

[1]: http://godoc.org/github.com/msteinert/pam
[2]: http://www.linux-pam.org/Linux-PAM-html/Linux-PAM_ADG.html
