# Minify <a name="minify"></a> [![Build Status](https://travis-ci.org/tdewolff/minify.svg?branch=master)](https://travis-ci.org/tdewolff/minify) [![GoDoc](http://godoc.org/github.com/tdewolff/minify?status.svg)](http://godoc.org/github.com/tdewolff/minify) [![Coverage Status](https://coveralls.io/repos/github/tdewolff/minify/badge.svg?branch=master)](https://coveralls.io/github/tdewolff/minify?branch=master) [![Join the chat at https://gitter.im/tdewolff/minify](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/tdewolff/minify?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

***BE AWARE: YOU NEED GO 1.9.7+, 1.10.3+, 1.11 to run the latest release!!!***

If you cannot upgrade Go, please pin to **minify@v2.3.6** and **parse@v2.3.4**

---

**[Online demo](https://go.tacodewolff.nl/minify) if you need to minify files *now*.**

**[Command line tool](https://github.com/tdewolff/minify/tree/master/cmd/minify) that minifies concurrently and supports watching file changes.**

**[All releases](https://github.com/tdewolff/minify/releases) for various platforms.**

---

*Did you know that the shortest valid piece of HTML5 is `<!doctype html><html lang=en><title>x</title>`? See for yourself at the [W3C Validator](http://validator.w3.org/)!*

Minify is a minifier package written in [Go][1]. It provides HTML5, CSS3, JS, JSON, SVG and XML minifiers and an interface to implement any other minifier. Minification is the process of removing bytes from a file (such as whitespace) without changing its output and therefore shrinking its size and speeding up transmission over the internet and possibly parsing. The implemented minifiers are designed for high performance.

The core functionality associates mimetypes with minification functions, allowing embedded resources (like CSS or JS within HTML files) to be minified as well. Users can add new implementations that are triggered based on a mimetype (or pattern), or redirect to an external command (like ClosureCompiler, UglifyCSS, ...).

#### Table of Contents

- [Minify](#minify)
	- [Prologue](#prologue)
	- [Installation](#installation)
	- [API stability](#api-stability)
	- [Testing](#testing)
	- [Performance](#performance)
	- [HTML](#html)
		- [Whitespace removal](#whitespace-removal)
	- [CSS](#css)
	- [JS](#js)
	- [JSON](#json)
	- [SVG](#svg)
	- [XML](#xml)
	- [Usage](#usage)
		- [New](#new)
		- [From reader](#from-reader)
		- [From bytes](#from-bytes)
		- [From string](#from-string)
		- [To reader](#to-reader)
		- [To writer](#to-writer)
		- [Middleware](#middleware)
		- [Custom minifier](#custom-minifier)
		- [Mediatypes](#mediatypes)
	- [Examples](#examples)
		- [Common minifiers](#common-minifiers)
		- [Custom minifier](#custom-minifier-example)
		- [ResponseWriter](#responsewriter)
		- [Templates](#templates)
	- [License](#license)

### Status

* CSS: **fully implemented**
* HTML: **fully implemented**
* JS: improved JSmin implementation
* JSON: **fully implemented**
* SVG: partially implemented; in development
* XML: **fully implemented**

### Roadmap

- [ ] Use ASM/SSE to further speed-up core parts of the parsers/minifiers
- [ ] Improve JS minifiers by shortening variables and proper semicolon omission
- [ ] Speed-up SVG minifier, it is very slow
- [x] Proper parser error reporting and line number + column information
- [ ] Generation of source maps (uncertain, might slow down parsers too much if it cannot run separately nicely)
- [ ] Create a cmd to pack webfiles (much like webpack), ie. merging CSS and JS files, inlining small external files, minification and gzipping. This would work on HTML files.

## Prologue
Minifiers or bindings to minifiers exist in almost all programming languages. Some implementations are merely using several regular expressions to trim whitespace and comments (even though regex for parsing HTML/XML is ill-advised, for a good read see [Regular Expressions: Now You Have Two Problems](http://blog.codinghorror.com/regular-expressions-now-you-have-two-problems/)). Some implementations are much more profound, such as the [YUI Compressor](http://yui.github.io/yuicompressor/) and [Google Closure Compiler](https://github.com/google/closure-compiler) for JS. As most existing implementations either use JavaScript, use regexes, and don't focus on performance, they are pretty slow.

This minifier proves to be that fast and extensive minifier that can handle HTML and any other filetype it may contain (CSS, JS, ...). It is usually orders of magnitude faster than existing minifiers.

## Installation
Run the following command

	go get -u github.com/tdewolff/minify/v2

or add the following imports and run the project with `go get`
``` go
import (
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
	"github.com/tdewolff/minify/v2/json"
	"github.com/tdewolff/minify/v2/svg"
	"github.com/tdewolff/minify/v2/xml"
)
```

## API stability
There is no guarantee for absolute stability, but I take issues and bugs seriously and don't take API changes lightly. The library will be maintained in a compatible way unless vital bugs prevent me from doing so. There has been one API change after v1 which added options support and I took the opportunity to push through some more API clean up as well. There are no plans whatsoever for future API changes.

## Testing
For all subpackages and the imported `parse` package, test coverage of 100% is pursued. Besides full coverage, the minifiers are [fuzz tested](https://github.com/tdewolff/fuzz) using [github.com/dvyukov/go-fuzz](http://www.github.com/dvyukov/go-fuzz), see [the wiki](https://github.com/tdewolff/minify/wiki) for the most important bugs found by fuzz testing. These tests ensure that everything works as intended and that the code does not crash (whatever the input). If you still encounter a bug, please file a [bug report](https://github.com/tdewolff/minify/issues)!

## Performance
The benchmarks directory contains a number of standardized samples used to compare performance between changes. To give an indication of the speed of this library, I've ran the tests on my Thinkpad T460 (i5-6300U quad-core 2.4GHz running Arch Linux) using Go 1.9.2.

```
name                              time/op
CSS/sample_bootstrap.css-4          2.26ms ± 0%
CSS/sample_gumby.css-4              2.92ms ± 1%
HTML/sample_amazon.html-4           2.33ms ± 2%
HTML/sample_bbc.html-4              1.02ms ± 1%
HTML/sample_blogpost.html-4          171µs ± 2%
HTML/sample_es6.html-4              14.5ms ± 0%
HTML/sample_stackoverflow.html-4    2.41ms ± 1%
HTML/sample_wikipedia.html-4        4.76ms ± 0%
JS/sample_ace.js-4                  7.41ms ± 0%
JS/sample_dot.js-4                  63.7µs ± 0%
JS/sample_jquery.js-4               2.99ms ± 0%
JS/sample_jqueryui.js-4             5.92ms ± 2%
JS/sample_moment.js-4               1.09ms ± 1%
JSON/sample_large.json-4            2.95ms ± 0%
JSON/sample_testsuite.json-4        1.51ms ± 1%
JSON/sample_twitter.json-4          6.75µs ± 1%
SVG/sample_arctic.svg-4             62.3ms ± 1%
SVG/sample_gopher.svg-4              218µs ± 0%
SVG/sample_usa.svg-4                33.1ms ± 3%
XML/sample_books.xml-4              36.2µs ± 0%
XML/sample_catalog.xml-4            14.9µs ± 0%
XML/sample_omg.xml-4                6.31ms ± 1%

name                              speed
CSS/sample_bootstrap.css-4        60.8MB/s ± 0%
CSS/sample_gumby.css-4            63.9MB/s ± 1%
HTML/sample_amazon.html-4          203MB/s ± 2%
HTML/sample_bbc.html-4             113MB/s ± 1%
HTML/sample_blogpost.html-4        123MB/s ± 2%
HTML/sample_es6.html-4            70.7MB/s ± 0%
HTML/sample_stackoverflow.html-4  85.2MB/s ± 1%
HTML/sample_wikipedia.html-4      93.6MB/s ± 0%
JS/sample_ace.js-4                86.9MB/s ± 0%
JS/sample_dot.js-4                81.0MB/s ± 0%
JS/sample_jquery.js-4             82.8MB/s ± 0%
JS/sample_jqueryui.js-4           79.3MB/s ± 2%
JS/sample_moment.js-4             91.2MB/s ± 1%
JSON/sample_large.json-4           258MB/s ± 0%
JSON/sample_testsuite.json-4       457MB/s ± 1%
JSON/sample_twitter.json-4         226MB/s ± 1%
SVG/sample_arctic.svg-4           23.6MB/s ± 1%
SVG/sample_gopher.svg-4           26.7MB/s ± 0%
SVG/sample_usa.svg-4              30.9MB/s ± 3%
XML/sample_books.xml-4             122MB/s ± 0%
XML/sample_catalog.xml-4           130MB/s ± 0%
XML/sample_omg.xml-4               180MB/s ± 1%
```

## HTML

HTML (with JS and CSS) minification typically shaves off about 10%.

The HTML5 minifier uses these minifications:

- strip unnecessary whitespace and otherwise collapse it to one space (or newline if it originally contained a newline)
- strip superfluous quotes, or uses single/double quotes whichever requires fewer escapes
- strip default attribute values and attribute boolean values
- strip some empty attributes
- strip unrequired tags (`html`, `head`, `body`, ...)
- strip unrequired end tags (`tr`, `td`, `li`, ... and often `p`)
- strip default protocols (`http:`, `https:` and `javascript:`)
- strip all comments (including conditional comments, old IE versions are not supported anymore by Microsoft)
- shorten `doctype` and `meta` charset
- lowercase tags, attributes and some values to enhance gzip compression

Options:

- `KeepConditionalComments` preserve all IE conditional comments such as `<!--[if IE 6]><![endif]-->` and `<![if IE 6]><![endif]>`, see https://msdn.microsoft.com/en-us/library/ms537512(v=vs.85).aspx#syntax
- `KeepDefaultAttrVals` preserve default attribute values such as `<script type="application/javascript">`
- `KeepDocumentTags` preserve `html`, `head` and `body` tags
- `KeepEndTags` preserve all end tags
- `KeepWhitespace` preserve whitespace between inline tags but still collapse multiple whitespace characters into one

After recent benchmarking and profiling it became really fast and minifies pages in the 10ms range, making it viable for on-the-fly minification.

However, be careful when doing on-the-fly minification. Minification typically trims off 10% and does this at worst around about 20MB/s. This means users have to download slower than 2MB/s to make on-the-fly minification worthwhile. This may or may not apply in your situation. Rather use caching!

### Whitespace removal
The whitespace removal mechanism collapses all sequences of whitespace (spaces, newlines, tabs) to a single space. If the sequence contained a newline or carriage return it will collapse into a newline character instead. It trims all text parts (in between tags) depending on whether it was preceded by a space from a previous piece of text and whether it is followed up by a block element or an inline element. In the former case we can omit spaces while for inline elements whitespace has significance.

Make sure your HTML doesn't depend on whitespace between `block` elements that have been changed to `inline` or `inline-block` elements using CSS. Your layout *should not* depend on those whitespaces as the minifier will remove them. An example is a menu consisting of multiple `<li>` that have `display:inline-block` applied and have whitespace in between them. It is bad practise to rely on whitespace for element positioning anyways!

## CSS

Minification typically shaves off about 10%-15%. This CSS minifier will _not_ do structural changes to your stylesheets. Although this could result in smaller files, the complexity is quite high and the risk of breaking website is high too.

The CSS minifier will only use safe minifications:

- remove comments and unnecessary whitespace (but keep `/*! ... */` which usually contains the license)
- remove trailing semicolons
- optimize `margin`, `padding` and `border-width` number of sides
- shorten numbers by removing unnecessary `+` and zeros and rewriting with/without exponent
- remove dimension and percentage for zero values
- remove quotes for URLs
- remove quotes for font families and make lowercase
- rewrite hex colors to/from color names, or to three digit hex
- rewrite `rgb(`, `rgba(`, `hsl(` and `hsla(` colors to hex or name
- use four digit hex for alpha values (`transparent` &#8594; `#0000`)
- replace `normal` and `bold` by numbers for `font-weight` and `font`
- replace `none` &#8594; `0` for `border`, `background` and `outline`
- lowercase all identifiers except classes, IDs and URLs to enhance gzip compression
- shorten MS alpha function
- rewrite data URIs with base64 or ASCII whichever is shorter
- calls minifier for data URI mediatypes, thus you can compress embedded SVG files if you have that minifier attached
- shorten aggregate declarations such as `background` and `font`

It does purposely not use the following techniques:

- (partially) merge rulesets
- (partially) split rulesets
- collapse multiple declarations when main declaration is defined within a ruleset (don't put `font-weight` within an already existing `font`, too complex)
- remove overwritten properties in ruleset (this not always overwrites it, for example with `!important`)
- rewrite properties into one ruleset if possible (like `margin-top`, `margin-right`, `margin-bottom` and `margin-left` &#8594; `margin`)
- put nested ID selector at the front (`body > div#elem p` &#8594; `#elem p`)
- rewrite attribute selectors for IDs and classes (`div[id=a]` &#8594; `div#a`)
- put space after pseudo-selectors (IE6 is old, move on!)

There are a couple of comparison tables online, such as [CSS Minifier Comparison](http://www.codenothing.com/benchmarks/css-compressor-3.0/full.html), [CSS minifiers comparison](http://www.phpied.com/css-minifiers-comparison/) and [CleanCSS tests](http://goalsmashers.github.io/css-minification-benchmark/). Comparing speed between each, this minifier will usually be between 10x-300x faster than existing implementations, and even rank among the top for minification ratios. It falls short with the purposely not implemented and often unsafe techniques.

Options:

- `Decimals` number of decimals to preserve for numbers, `-1` means no trimming
- `KeepCSS2` prohibits using CSS3 syntax (such as exponents in numbers, or `rgba(` &#8594; `rgb(`), might be incomplete

## JS

The JS minifier is pretty basic. It removes comments, whitespace and line breaks whenever it can. It employs all the rules that [JSMin](http://www.crockford.com/javascript/jsmin.html) does too, but has additional improvements. For example the prefix-postfix bug is fixed.

Common speeds of PHP and JS implementations are about 100-300kB/s (see [Uglify2](http://lisperator.net/uglifyjs/), [Adventures in PHP web asset minimization](https://www.happyassassin.net/2014/12/29/adventures-in-php-web-asset-minimization/)). This implementation or orders of magnitude faster, around ~80MB/s.

TODO:
- shorten local variables / function parameters names
- precise semicolon and newline omission

## JSON

Minification typically shaves off about 15% of filesize for common indented JSON such as generated by [JSON Generator](http://www.json-generator.com/).

The JSON minifier only removes whitespace, which is the only thing that can be left out.

## SVG

The SVG minifier uses these minifications:

- trim and collapse whitespace between all tags
- strip comments, empty `doctype`, XML prelude, `metadata`
- strip SVG version
- strip CDATA sections wherever possible
- collapse tags with no content to a void tag
- minify style tag and attributes with the CSS minifier
- minify colors
- shorten lengths and numbers and remove default `px` unit
- shorten `path` data
- convert `rect`, `line`, `polygon`, `polyline` to `path`
- use relative or absolute positions in path data whichever is shorter

TODO:
- convert attributes to style attribute whenever shorter
- merge path data? (same style and no intersection -- the latter is difficult)

Options:

- `Decimals` number of decimals to preserve for numbers, `-1` means no trimming

## XML

The XML minifier uses these minifications:

- strip unnecessary whitespace and otherwise collapse it to one space (or newline if it originally contained a newline)
- strip comments
- collapse tags with no content to a void tag
- strip CDATA sections wherever possible

Options:

- `KeepWhitespace` preserve whitespace between inline tags but still collapse multiple whitespace characters into one

## Usage
Any input stream is being buffered by the minification functions. This is how the underlying buffer package inherently works to ensure high performance. The output stream however is not buffered. It is wise to preallocate a buffer as big as the input to which the output is written, or otherwise use `bufio` to buffer to a streaming writer.

### New
Retrieve a minifier struct which holds a map of mediatype &#8594; minifier functions.
``` go
m := minify.New()
```

The following loads all provided minifiers.
``` go
m := minify.New()
m.AddFunc("text/css", css.Minify)
m.AddFunc("text/html", html.Minify)
m.AddFunc("image/svg+xml", svg.Minify)
m.AddFuncRegexp(regexp.MustCompile("^(application|text)/(x-)?(java|ecma)script$"), js.Minify)
m.AddFuncRegexp(regexp.MustCompile("[/+]json$"), json.Minify)
m.AddFuncRegexp(regexp.MustCompile("[/+]xml$"), xml.Minify)
```

You can set options to several minifiers.
``` go
m.Add("text/html", &html.Minifier{
	KeepDefaultAttrVals: true,
	KeepWhitespace: true,
})
```

### From reader
Minify from an `io.Reader` to an `io.Writer` for a specific mediatype.
``` go
if err := m.Minify(mediatype, w, r); err != nil {
	panic(err)
}
```

### From bytes
Minify from and to a `[]byte` for a specific mediatype.
``` go
b, err = m.Bytes(mediatype, b)
if err != nil {
	panic(err)
}
```

### From string
Minify from and to a `string` for a specific mediatype.
``` go
s, err = m.String(mediatype, s)
if err != nil {
	panic(err)
}
```

### To reader
Get a minifying reader for a specific mediatype.
``` go
mr := m.Reader(mediatype, r)
if _, err := mr.Read(b); err != nil {
	panic(err)
}
```

### To writer
Get a minifying writer for a specific mediatype. Must be explicitly closed because it uses an `io.Pipe` underneath.
``` go
mw := m.Writer(mediatype, w)
if mw.Write([]byte("input")); err != nil {
	panic(err)
}
if err := mw.Close(); err != nil {
	panic(err)
}
```

### Middleware
Minify resources on the fly using middleware. It passes a wrapped response writer to the handler that removes the Content-Length header. The minifier is chosen based on the Content-Type header or, if the header is empty, by the request URI file extension. This is on-the-fly processing, you should preferably cache the results though!
``` go
fs := http.FileServer(http.Dir("www/"))
http.Handle("/", m.Middleware(fs))
```

### Custom minifier
Add a minifier for a specific mimetype.
``` go
type CustomMinifier struct {
	KeepLineBreaks bool
}

func (c *CustomMinifier) Minify(m *minify.M, w io.Writer, r io.Reader, params map[string]string) error {
	// ...
	return nil
}

m.Add(mimetype, &CustomMinifier{KeepLineBreaks: true})
// or
m.AddRegexp(regexp.MustCompile("/x-custom$"), &CustomMinifier{KeepLineBreaks: true})
```

Add a minify function for a specific mimetype.
``` go
m.AddFunc(mimetype, func(m *minify.M, w io.Writer, r io.Reader, params map[string]string) error {
	// ...
	return nil
})
m.AddFuncRegexp(regexp.MustCompile("/x-custom$"), func(m *minify.M, w io.Writer, r io.Reader, params map[string]string) error {
	// ...
	return nil
})
```

Add a command `cmd` with arguments `args` for a specific mimetype.
``` go
m.AddCmd(mimetype, exec.Command(cmd, args...))
m.AddCmdRegexp(regexp.MustCompile("/x-custom$"), exec.Command(cmd, args...))
```

### Mediatypes
Using the `params map[string]string` argument one can pass parameters to the minifier such as seen in mediatypes (`type/subtype; key1=val2; key2=val2`). Examples are the encoding or charset of the data. Calling `Minify` will split the mimetype and parameters for the minifiers for you, but `MinifyMimetype` can be used if you already have them split up.

Minifiers can also be added using a regular expression. For example a minifier with `image/.*` will match any image mime.

## Examples
### Common minifiers
Basic example that minifies from stdin to stdout and loads the default HTML, CSS and JS minifiers. Optionally, one can enable `java -jar build/compiler.jar` to run for JS (for example the [ClosureCompiler](https://code.google.com/p/closure-compiler/)). Note that reading the file into a buffer first and writing to a pre-allocated buffer would be faster (but would disable streaming).
``` go
package main

import (
	"log"
	"os"
	"os/exec"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
	"github.com/tdewolff/minify/v2/json"
	"github.com/tdewolff/minify/v2/svg"
	"github.com/tdewolff/minify/v2/xml"
)

func main() {
	m := minify.New()
	m.AddFunc("text/css", css.Minify)
	m.AddFunc("text/html", html.Minify)
	m.AddFunc("image/svg+xml", svg.Minify)
	m.AddFuncRegexp(regexp.MustCompile("^(application|text)/(x-)?(java|ecma)script$"), js.Minify)
	m.AddFuncRegexp(regexp.MustCompile("[/+]json$"), json.Minify)
	m.AddFuncRegexp(regexp.MustCompile("[/+]xml$"), xml.Minify)

	// Or use the following for better minification of JS but lower speed:
	// m.AddCmdRegexp(regexp.MustCompile("^(application|text)/(x-)?(java|ecma)script$"), exec.Command("java", "-jar", "build/compiler.jar"))

	if err := m.Minify("text/html", os.Stdout, os.Stdin); err != nil {
		panic(err)
	}
}
```

### <a name="custom-minifier-example"></a> Custom minifier
Custom minifier showing an example that implements the minifier function interface. Within a custom minifier, it is possible to call any minifier function (through `m minify.Minifier`) recursively when dealing with embedded resources.
``` go
package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/tdewolff/minify/v2"
)

func main() {
	m := minify.New()
	m.AddFunc("text/plain", func(m *minify.M, w io.Writer, r io.Reader, _ map[string]string) error {
		// remove newlines and spaces
		rb := bufio.NewReader(r)
		for {
			line, err := rb.ReadString('\n')
			if err != nil && err != io.EOF {
				return err
			}
			if _, errws := io.WriteString(w, strings.Replace(line, " ", "", -1)); errws != nil {
				return errws
			}
			if err == io.EOF {
				break
			}
		}
		return nil
	})

	in := "Because my coffee was too cold, I heated it in the microwave."
	out, err := m.String("text/plain", in)
	if err != nil {
		panic(err)
	}
	fmt.Println(out)
	// Output: Becausemycoffeewastoocold,Iheateditinthemicrowave.
}
```

### ResponseWriter
#### Middleware
``` go
func main() {
	m := minify.New()
	m.AddFunc("text/css", css.Minify)
	m.AddFunc("text/html", html.Minify)
	m.AddFunc("image/svg+xml", svg.Minify)
	m.AddFuncRegexp(regexp.MustCompile("^(application|text)/(x-)?(java|ecma)script$"), js.Minify)
	m.AddFuncRegexp(regexp.MustCompile("[/+]json$"), json.Minify)
	m.AddFuncRegexp(regexp.MustCompile("[/+]xml$"), xml.Minify)

	fs := http.FileServer(http.Dir("www/"))
	http.Handle("/", m.Middleware(fs))
}
```

#### ResponseWriter
``` go
func Serve(w http.ResponseWriter, r *http.Request) {
	mw := m.ResponseWriter(w, r)
	defer mw.Close()
	w = mw

	http.ServeFile(w, r, path.Join("www", r.URL.Path))
}
```

#### Custom response writer
ResponseWriter example which returns a ResponseWriter that minifies the content and then writes to the original ResponseWriter. Any write after applying this filter will be minified.
``` go
type MinifyResponseWriter struct {
	http.ResponseWriter
	io.WriteCloser
}

func (m MinifyResponseWriter) Write(b []byte) (int, error) {
	return m.WriteCloser.Write(b)
}

// MinifyResponseWriter must be closed explicitly by calling site.
func MinifyFilter(mediatype string, res http.ResponseWriter) MinifyResponseWriter {
	m := minify.New()
	// add minfiers

	mw := m.Writer(mediatype, res)
	return MinifyResponseWriter{res, mw}
}
```

``` go
// Usage
func(w http.ResponseWriter, req *http.Request) {
	w = MinifyFilter("text/html", w)
	if _, err := io.WriteString(w, "<p class="message"> This HTTP response will be minified. </p>"); err != nil {
		panic(err)
	}
	if err := w.Close(); err != nil {
		panic(err)
	}
	// Output: <p class=message>This HTTP response will be minified.
}
```

### Templates

Here's an example of a replacement for `template.ParseFiles` from `template/html`, which automatically minifies each template before parsing it.

Be aware that minifying templates will work in most cases but not all. Because the HTML minifier only works for valid HTML5, your template must be valid HTML5 of itself. Template tags are parsed as regular text by the minifier.

``` go
func compileTemplates(filenames ...string) (*template.Template, error) {
	m := minify.New()
	m.AddFunc("text/html", html.Minify)

	var tmpl *template.Template
	for _, filename := range filenames {
		name := filepath.Base(filename)
		if tmpl == nil {
			tmpl = template.New(name)
		} else {
			tmpl = tmpl.New(name)
		}

		b, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, err
		}

		mb, err := m.Bytes("text/html", b)
		if err != nil {
			return nil, err
		}
		tmpl.Parse(string(mb))
	}
	return tmpl, nil
}
```

Example usage:

``` go
templates := template.Must(compileTemplates("view.html", "home.html"))
```

## License
Released under the [MIT license](LICENSE.md).

[1]: http://golang.org/ "Go Language"
