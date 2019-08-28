# XML [![GoDoc](http://godoc.org/github.com/tdewolff/parse/xml?status.svg)](http://godoc.org/github.com/tdewolff/parse/xml)

This package is an XML lexer written in [Go][1]. It follows the specification at [Extensible Markup Language (XML) 1.0 (Fifth Edition)](http://www.w3.org/TR/REC-xml/). The lexer takes an io.Reader and converts it into tokens until the EOF.

## Installation
Run the following command

	go get -u github.com/tdewolff/parse/v2/xml

or add the following import and run project with `go get`

	import "github.com/tdewolff/parse/v2/xml"

## Lexer
### Usage
The following initializes a new Lexer with io.Reader `r`:
``` go
l := xml.NewLexer(r)
```

To tokenize until EOF an error, use:
``` go
for {
	tt, data := l.Next()
	switch tt {
	case xml.ErrorToken:
		// error or EOF set in l.Err()
		return
	case xml.StartTagToken:
		// ...
		for {
			ttAttr, dataAttr := l.Next()
			if ttAttr != xml.AttributeToken {
				// handle StartTagCloseToken/StartTagCloseVoidToken/StartTagClosePIToken
				break
			}
			// ...
		}
	case xml.EndTagToken:
		// ...
	}
}
```

All tokens:
``` go
ErrorToken TokenType = iota // extra token when errors occur
CommentToken
CDATAToken
StartTagToken
StartTagCloseToken
StartTagCloseVoidToken
StartTagClosePIToken
EndTagToken
AttributeToken
TextToken
```

### Examples
``` go
package main

import (
	"os"

	"github.com/tdewolff/parse/v2/xml"
)

// Tokenize XML from stdin.
func main() {
	l := xml.NewLexer(os.Stdin)
	for {
		tt, data := l.Next()
		switch tt {
		case xml.ErrorToken:
			if l.Err() != io.EOF {
				fmt.Println("Error on line", l.Line(), ":", l.Err())
			}
			return
		case xml.StartTagToken:
			fmt.Println("Tag", string(data))
			for {
				ttAttr, dataAttr := l.Next()
				if ttAttr != xml.AttributeToken {
					break
				}

				key := dataAttr
				val := l.AttrVal()
				fmt.Println("Attribute", string(key), "=", string(val))
			}
		// ...
		}
	}
}
```

## License
Released under the [MIT license](https://github.com/tdewolff/parse/blob/master/LICENSE.md).

[1]: http://golang.org/ "Go Language"
