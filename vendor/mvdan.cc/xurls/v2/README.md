# xurls

[![GoDoc](https://godoc.org/mvdan.cc/xurls?status.svg)](https://godoc.org/mvdan.cc/xurls)

Extract urls from text using regular expressions. Requires Go 1.13 or later.

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

Since API is centered around [regexp.Regexp](https://golang.org/pkg/regexp/#Regexp),
many other methods are available, such as finding the [byte indexes](https://golang.org/pkg/regexp/#Regexp.FindAllIndex)
for all matches.

Note that calling the exposed functions means compiling a regular expression, so
repeated calls should be avoided.

#### cmd/xurls

To install the tool globally:

	cd $(mktemp -d); go mod init tmp; GO111MODULE=on go get mvdan.cc/xurls/v2/cmd/xurls

```shell
$ echo "Do gophers live in http://golang.org?" | xurls
http://golang.org
```
