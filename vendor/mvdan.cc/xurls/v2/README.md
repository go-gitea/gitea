# xurls

[![GoDoc](https://godoc.org/mvdan.cc/xurls?status.svg)](https://godoc.org/mvdan.cc/xurls)
[![Travis](https://travis-ci.org/mvdan/xurls.svg?branch=master)](https://travis-ci.org/mvdan/xurls)

Extract urls from text using regular expressions. Requires Go 1.10.3 or later.

```go
import "mvdan.cc/xurls/v2"

func main() {
	xurls.Relaxed().FindString("Do gophers live in golang.org?")
	// "golang.org"
	xurls.Strict().FindAllString("foo.com is http://foo.com/.", -1)
	// []string{"http://foo.com/"}
}
```

Note that the funcs compile regexes, so avoid calling them repeatedly.

#### cmd/xurls

	go get -u mvdan.cc/xurls/v2/cmd/xurls

```shell
$ echo "Do gophers live in http://golang.org?" | xurls
http://golang.org
```
