# Parse [![Build Status](https://travis-ci.org/tdewolff/parse.svg?branch=master)](https://travis-ci.org/tdewolff/parse) [![GoDoc](http://godoc.org/github.com/tdewolff/parse?status.svg)](http://godoc.org/github.com/tdewolff/parse) [![Coverage Status](https://coveralls.io/repos/github/tdewolff/parse/badge.svg?branch=master)](https://coveralls.io/github/tdewolff/parse?branch=master)

***BE AWARE: YOU NEED GO 1.9.7+, 1.10.3+, 1.11 to run the latest release!!!***

If you cannot upgrade Go, please pin to **parse@v2.3.4**

---

This package contains several lexers and parsers written in [Go][1]. All subpackages are built to be streaming, high performance and to be in accordance with the official (latest) specifications.

The lexers are implemented using `buffer.Lexer` in https://github.com/tdewolff/parse/buffer and the parsers work on top of the lexers. Some subpackages have hashes defined (using [Hasher](https://github.com/tdewolff/hasher)) that speed up common byte-slice comparisons.

## Buffer
### Reader
Reader is a wrapper around a `[]byte` that implements the `io.Reader` interface. It is comparable to `bytes.Reader` but has slightly different semantics (and a slightly smaller memory footprint).

### Writer
Writer is a buffer that implements the `io.Writer` interface and expands the buffer as needed. The reset functionality allows for better memory reuse. After calling `Reset`, it will overwrite the current buffer and thus reduce allocations.

### Lexer
Lexer is a read buffer specifically designed for building lexers. It keeps track of two positions: a start and end position. The start position is the beginning of the current token being parsed, the end position is being moved forward until a valid token is found. Calling `Shift` will collapse the positions to the end and return the parsed `[]byte`.

Moving the end position can go through `Move(int)` which also accepts negative integers. One can also use `Pos() int` to try and parse a token, and if it fails rewind with `Rewind(int)`, passing the previously saved position.

`Peek(int) byte` will peek forward (relative to the end position) and return the byte at that location. `PeekRune(int) (rune, int)` returns UTF-8 runes and its length at the given **byte** position. Upon an error `Peek` will return `0`, the **user must peek at every character** and not skip any, otherwise it may skip a `0` and panic on out-of-bounds indexing.

`Lexeme() []byte` will return the currently selected bytes, `Skip()` will collapse the selection. `Shift() []byte` is a combination of `Lexeme() []byte` and `Skip()`.

When the passed `io.Reader` returned an error, `Err() error` will return that error even if not at the end of the buffer.

### StreamLexer
StreamLexer behaves like Lexer but uses a buffer pool to read in chunks from `io.Reader`, retaining old buffers in memory that are still in use, and re-using old buffers otherwise. Calling `Free(n int)` frees up `n` bytes from the internal buffer(s). It holds an array of buffers to accommodate for keeping everything in-memory. Calling `ShiftLen() int` returns the number of bytes that have been shifted since the previous call to `ShiftLen`, which can be used to specify how many bytes need to be freed up from the buffer. If you don't need to keep returned byte slices around, call `Free(ShiftLen())` after every `Shift` call.

## Strconv
This package contains string conversion function much like the standard library's `strconv` package, but it is specifically tailored for the performance needs within the `minify` package.

For example, the floating-point to string conversion function is approximately twice as fast as the standard library, but it is not as precise.

## CSS
This package is a CSS3 lexer and parser. Both follow the specification at [CSS Syntax Module Level 3](http://www.w3.org/TR/css-syntax-3/). The lexer takes an io.Reader and converts it into tokens until the EOF. The parser returns a parse tree of the full io.Reader input stream, but the low-level `Next` function can be used for stream parsing to returns grammar units until the EOF.

[See README here](https://github.com/tdewolff/parse/tree/master/css).

## HTML
This package is an HTML5 lexer. It follows the specification at [The HTML syntax](http://www.w3.org/TR/html5/syntax.html). The lexer takes an io.Reader and converts it into tokens until the EOF.

[See README here](https://github.com/tdewolff/parse/tree/master/html).

## JS
This package is a JS lexer (ECMA-262, edition 6.0). It follows the specification at [ECMAScript Language Specification](http://www.ecma-international.org/ecma-262/6.0/). The lexer takes an io.Reader and converts it into tokens until the EOF.

[See README here](https://github.com/tdewolff/parse/tree/master/js).

## JSON
This package is a JSON parser (ECMA-404). It follows the specification at [JSON](http://json.org/). The parser takes an io.Reader and converts it into tokens until the EOF.

[See README here](https://github.com/tdewolff/parse/tree/master/json).

## SVG
This package contains common hashes for SVG1.1 tags and attributes.

## XML
This package is an XML1.0 lexer. It follows the specification at [Extensible Markup Language (XML) 1.0 (Fifth Edition)](http://www.w3.org/TR/xml/). The lexer takes an io.Reader and converts it into tokens until the EOF.

[See README here](https://github.com/tdewolff/parse/tree/master/xml).

## License
Released under the [MIT license](LICENSE.md).

[1]: http://golang.org/ "Go Language"
