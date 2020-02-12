# substring [![Build Status](https://travis-ci.org/toqueteos/substring.png?branch=master)](https://travis-ci.org/toqueteos/substring) [![GoDoc](http://godoc.org/github.com/toqueteos/substring?status.png)](http://godoc.org/github.com/toqueteos/substring) [![GitHub release](https://img.shields.io/github/release/toqueteos/substring.svg)](https://github.com/toqueteos/substring/releases)

Simple and composable alternative to [regexp](http://golang.org/pkg/regexp/) package for fast substring searches.

## Installation

The recommended way to install substring

```
go get -t gopkg.in/toqueteos/substring.v1
```

The `-t` flag is for fetching [gocheck](https://gopkg.in/check.v1), required for tests and benchmarks.

## Examples

A basic example with two matchers:

```go
package main

import (
    "fmt"
    "regexp"

    "gopkg.in/toqueteos/substring.v1"
)

func main() {
    m1 := substring.After("assets/", substring.Or(
        substring.Has("jquery"),
        substring.Has("angular"),
        substring.Suffixes(".js", ".css", ".html"),
    ))
    fmt.Println(m1.Match("assets/angular/foo/bar")) //Prints: true
    fmt.Println(m1.Match("assets/js/file.js"))      //Prints: true
    fmt.Println(m1.Match("assets/style/bar.css"))   //Prints: true
    fmt.Println(m1.Match("assets/foo/bar.html"))    //Prints: false
    fmt.Println(m1.Match("assets/js/qux.json"))     //Prints: false
    fmt.Println(m1.Match("core/file.html"))         //Prints: false
    fmt.Println(m1.Match("foobar/that.jsx"))        //Prints: false

    m2 := substring.After("vendor/", substring.Suffixes(".css", ".js", ".less"))

    fmt.Println(m2.Match("foo/vendor/bar/qux.css")) //Prints: true
    fmt.Println(m2.Match("foo/var/qux.less"))       //Prints: false

    re := regexp.MustCompile(`vendor\/.*\.(css|js|less)$`)
    fmt.Println(re.MatchString("foo/vendor/bar/qux.css")) //Prints: true
    fmt.Println(re.MatchString("foo/var/qux.less"))       //Prints: false
}
```

## How fast?

It may vary depending on your use case but 1~2 orders of magnitude faster than `regexp` is pretty common.

Test it out for yourself by running `go test -check.b`!

```
$ go test -check.b
PASS: lib_test.go:18: LibSuite.BenchmarkExample1        10000000               221 ns/op
PASS: lib_test.go:23: LibSuite.BenchmarkExample2        10000000               229 ns/op
PASS: lib_test.go:28: LibSuite.BenchmarkExample3        10000000               216 ns/op
PASS: lib_test.go:33: LibSuite.BenchmarkExample4        10000000               208 ns/op
PASS: lib_test.go:38: LibSuite.BenchmarkExample5        20000000                82.1 ns/op
PASS: lib_test.go:48: LibSuite.BenchmarkExampleRe1        500000              4136 ns/op
PASS: lib_test.go:53: LibSuite.BenchmarkExampleRe2        500000              5222 ns/op
PASS: lib_test.go:58: LibSuite.BenchmarkExampleRe3        500000              5116 ns/op
PASS: lib_test.go:63: LibSuite.BenchmarkExampleRe4        500000              4020 ns/op
PASS: lib_test.go:68: LibSuite.BenchmarkExampleRe5      10000000               226 ns/op
OK: 10 passed
PASS
ok      gopkg.in/toqueteos/substring.v1 23.471s
```

License
-------

MIT, see [LICENSE](LICENSE)
