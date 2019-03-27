# html2text

[![Documentation](https://godoc.org/github.com/jaytaylor/html2text?status.svg)](https://godoc.org/github.com/jaytaylor/html2text)
[![Build Status](https://travis-ci.org/jaytaylor/html2text.svg?branch=master)](https://travis-ci.org/jaytaylor/html2text)
[![Report Card](https://goreportcard.com/badge/github.com/jaytaylor/html2text)](https://goreportcard.com/report/github.com/jaytaylor/html2text)

### Converts HTML into text


## Introduction

html2text is a simple golang package for rendering HTML into plaintext.

There are still lots of improvements to be had, but FWIW this has worked fine for my [basic] HTML-2-text needs.

It requires go 1.x or newer ;)


## Download the package

```bash
go get github.com/jaytaylor/html2text
```

## Example usage

```go
package main

import (
	"fmt"

	"github.com/jaytaylor/html2text"
)

func main() {
	inputHtml := `
          <html>
            <head>
              <title>My Mega Service</title>
              <link rel=\"stylesheet\" href=\"main.css\">
              <style type=\"text/css\">body { color: #fff; }</style>
            </head>
        
            <body>
              <div class="logo">
                <a href="http://mymegaservice.com/"><img src="/logo-image.jpg" alt="Mega Service"/></a>
              </div>
        
              <h1>Welcome to your new account on my service!</h1>
        
              <p>
                  Here is some more information:
        
                  <ul>
                      <li>Link 1: <a href="https://example.com">Example.com</a></li>
                      <li>Link 2: <a href="https://example2.com">Example2.com</a></li>
                      <li>Something else</li>
                  </ul>
              </p>
            </body>
          </html>
	`

	text, err := html2text.FromString(inputHtml)
	if err != nil {
		panic(err)
	}
	fmt.Println(text)
}
```

Output:
```
Mega Service ( http://mymegaservice.com/ )

******************************************
Welcome to your new account on my service!
******************************************

Here is some more information:

* Link 1: Example.com ( https://example.com )
* Link 2: Example2.com ( https://example2.com )
* Something else
```


## Unit-tests

Running the unit-tests is straightforward and standard:

```bash
go test
```


# License

Permissive MIT license.


## Contact

You are more than welcome to open issues and send pull requests if you find a bug or want a new feature.

If you appreciate this library please feel free to drop me a line and tell me!  It's always nice to hear from people who have benefitted from my work.

Email: jay at (my github username).com

Twitter: [@jtaylor](https://twitter.com/jtaylor)

