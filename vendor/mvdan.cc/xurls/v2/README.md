# xurls

[![GoDoc](https://godoc.org/mvdan.cc/xurls?status.svg)](https://godoc.org/mvdan.cc/xurls)

Extract urls from text using regular expressions. Requires Go 1.12 or later.

```go
import "mvdan.cc/xurls/v2"

func main() {
	rxRelaxed := xurls.Relaxed()
	rxRelaxed.FindString("Do gophers live in golang.org?")  // "golang.org"
	rxRelaxed.FindString("This string does not have a URL") // ""

	rxStrict := xurls.Strict()
	rxStrict.FindAllString("must have scheme: http://foo.com/.", -1) // []string{"http://foo.com/"}
	rxStrict.FindAllString("no scheme, no match: foo.com", -1)       // []string{}
}
```

Note that the funcs compile regexes, so avoid calling them repeatedly.

#### cmd/xurls

To install the tool globally:

	go get mvdan.cc/xurls/cmd/xurls

```shell
$ echo "Do gophers live in http://golang.org?" | xurls
http://golang.org
```
