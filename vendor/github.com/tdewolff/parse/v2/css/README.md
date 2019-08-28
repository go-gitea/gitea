# CSS [![GoDoc](http://godoc.org/github.com/tdewolff/parse/css?status.svg)](http://godoc.org/github.com/tdewolff/parse/css)

This package is a CSS3 lexer and parser written in [Go][1]. Both follow the specification at [CSS Syntax Module Level 3](http://www.w3.org/TR/css-syntax-3/). The lexer takes an io.Reader and converts it into tokens until the EOF. The parser returns a parse tree of the full io.Reader input stream, but the low-level `Next` function can be used for stream parsing to returns grammar units until the EOF.

## Installation
Run the following command

	go get -u github.com/tdewolff/parse/v2/css

or add the following import and run project with `go get`

	import "github.com/tdewolff/parse/v2/css"

## Lexer
### Usage
The following initializes a new Lexer with io.Reader `r`:
``` go
l := css.NewLexer(r)
```

To tokenize until EOF an error, use:
``` go
for {
	tt, text := l.Next()
	switch tt {
	case css.ErrorToken:
		// error or EOF set in l.Err()
		return
	// ...
	}
}
```

All tokens (see [CSS Syntax Module Level 3](http://www.w3.org/TR/css3-syntax/)):
``` go
ErrorToken			// non-official token, returned when errors occur
IdentToken
FunctionToken		// rgb( rgba( ...
AtKeywordToken		// @abc
HashToken			// #abc
StringToken
BadStringToken
UrlToken			// url(
BadUrlToken
DelimToken			// any unmatched character
NumberToken			// 5
PercentageToken		// 5%
DimensionToken		// 5em
UnicodeRangeToken
IncludeMatchToken	// ~=
DashMatchToken		// |=
PrefixMatchToken	// ^=
SuffixMatchToken	// $=
SubstringMatchToken // *=
ColumnToken			// ||
WhitespaceToken
CDOToken 			// <!--
CDCToken 			// -->
ColonToken
SemicolonToken
CommaToken
BracketToken 		// ( ) [ ] { }, all bracket tokens use this, Data() can distinguish between the brackets
CommentToken		// non-official token
```

### Examples
``` go
package main

import (
	"os"

	"github.com/tdewolff/parse/v2/css"
)

// Tokenize CSS3 from stdin.
func main() {
	l := css.NewLexer(os.Stdin)
	for {
		tt, text := l.Next()
		switch tt {
		case css.ErrorToken:
			if l.Err() != io.EOF {
				fmt.Println("Error on line", l.Line(), ":", l.Err())
			}
			return
		case css.IdentToken:
			fmt.Println("Identifier", string(text))
		case css.NumberToken:
			fmt.Println("Number", string(text))
		// ...
		}
	}
}
```

## Parser
### Usage
The following creates a new Parser.
``` go
// true because this is the content of an inline style attribute
p := css.NewParser(bytes.NewBufferString("color: red;"), true)
```

To iterate over the stylesheet, use:
``` go
for {
    gt, _, data := p.Next()
    if gt == css.ErrorGrammar {
        break
    }
    // ...
}
```

All grammar units returned by `Next`:
``` go
ErrorGrammar
AtRuleGrammar
EndAtRuleGrammar
RulesetGrammar
EndRulesetGrammar
DeclarationGrammar
TokenGrammar
```

### Examples
``` go
package main

import (
	"bytes"
	"fmt"

	"github.com/tdewolff/parse/v2/css"
)

func main() {
	// true because this is the content of an inline style attribute
	p := css.NewParser(bytes.NewBufferString("color: red;"), true)
	out := ""
	for {
		gt, _, data := p.Next()
		if gt == css.ErrorGrammar {
			break
		} else if gt == css.AtRuleGrammar || gt == css.BeginAtRuleGrammar || gt == css.BeginRulesetGrammar || gt == css.DeclarationGrammar {
			out += string(data)
			if gt == css.DeclarationGrammar {
				out += ":"
			}
			for _, val := range p.Values() {
				out += string(val.Data)
			}
			if gt == css.BeginAtRuleGrammar || gt == css.BeginRulesetGrammar {
				out += "{"
			} else if gt == css.AtRuleGrammar || gt == css.DeclarationGrammar {
				out += ";"
			}
		} else {
			out += string(data)
		}
	}
	fmt.Println(out)
}

```

## License
Released under the [MIT license](https://github.com/tdewolff/parse/blob/master/LICENSE.md).

[1]: http://golang.org/ "Go Language"
