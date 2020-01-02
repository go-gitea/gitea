goldmark
==========================================

[![http://godoc.org/github.com/yuin/goldmark](https://godoc.org/github.com/yuin/goldmark?status.svg)](http://godoc.org/github.com/yuin/goldmark)
[![https://github.com/yuin/goldmark/actions?query=workflow:test](https://github.com/yuin/goldmark/workflows/test/badge.svg?branch=master&event=push)](https://github.com/yuin/goldmark/actions?query=workflow:test)
[![https://coveralls.io/github/yuin/goldmark](https://coveralls.io/repos/github/yuin/goldmark/badge.svg?branch=master)](https://coveralls.io/github/yuin/goldmark)
[![https://goreportcard.com/report/github.com/yuin/goldmark](https://goreportcard.com/badge/github.com/yuin/goldmark)](https://goreportcard.com/report/github.com/yuin/goldmark)

> A Markdown parser written in Go. Easy to extend, standard compliant, well structured.

goldmark is compliant with CommonMark 0.29.

Motivation
----------------------
I need a Markdown parser for Go that meets following conditions:

- Easy to extend.
    - Markdown is poor in document expressions compared with other light markup languages like reStructuredText.
    - We have extensions to the Markdown syntax, e.g. PHP Markdown Extra, GitHub Flavored Markdown.
- Standard compliant.
    - Markdown has many dialects.
    - GitHub Flavored Markdown is widely used and it is based on CommonMark aside from whether CommonMark is good specification or not.
        - CommonMark is too complicated and hard to implement.
- Well structured.
    - AST based, and preserves source position of nodes.
- Written in pure Go.

[golang-commonmark](https://gitlab.com/golang-commonmark/markdown) may be a good choice, but it seems to be a copy of [markdown-it](https://github.com/markdown-it).

[blackfriday.v2](https://github.com/russross/blackfriday/tree/v2) is a fast and widely used implementation, but it is not CommonMark compliant and cannot be extended from outside of the package since its AST uses structs instead of interfaces.

Furthermore, its behavior differs from other implementations in some cases, especially regarding lists: ([Deep nested lists don't output correctly #329](https://github.com/russross/blackfriday/issues/329), [List block cannot have a second line #244](https://github.com/russross/blackfriday/issues/244), etc).

This behavior sometimes causes problems. If you migrate your Markdown text to blackfriday-based wikis from GitHub, many lists will immediately be broken.

As mentioned above, CommonMark is too complicated and hard to implement, so Markdown parsers based on CommonMark barely exist.

Features
----------------------

- **Standard compliant.**  goldmark gets full compliance with the latest CommonMark spec.
- **Extensible.**  Do you want to add a `@username` mention syntax to Markdown?
  You can easily do it in goldmark. You can add your AST nodes,
  parsers for block level elements, parsers for inline level elements,
  transformers for paragraphs, transformers for whole AST structure, and
  renderers.
- **Performance.**  goldmark performs pretty much equally to cmark,
  the CommonMark reference implementation written in C.
- **Robust.**  goldmark is tested with [go-fuzz](https://github.com/dvyukov/go-fuzz), a fuzz testing tool.
- **Builtin extensions.**  goldmark ships with common extensions like tables, strikethrough,
  task lists, and definition lists.
- **Depends only on standard libraries.**

Installation
----------------------
```bash
$ go get github.com/yuin/goldmark
```


Usage
----------------------
Import packages:

```
import (
	"bytes"
	"github.com/yuin/goldmark"
)
```


Convert Markdown documents with the CommonMark compliant mode:

```go
var buf bytes.Buffer
if err := goldmark.Convert(source, &buf); err != nil {
  panic(err)
}
```

With options
------------------------------

```go
var buf bytes.Buffer
if err := goldmark.Convert(source, &buf, parser.WithContext(ctx)); err != nil {
  panic(err)
}
```

| Functional option | Type | Description |
| ----------------- | ---- | ----------- |
| `parser.WithContext` | A `parser.Context` | Context for the parsing phase. |

Context options
----------------------

| Functional option | Type | Description |
| ----------------- | ---- | ----------- |
| `parser.WithIDs` | A `parser.IDs` | `IDs` allows you to change logics that are related to element id(ex: Auto heading id generation). |


Custom parser and renderer
--------------------------
```go
import (
	"bytes"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

md := goldmark.New(
          goldmark.WithExtensions(extension.GFM),
          goldmark.WithParserOptions(
              parser.WithAutoHeadingID(),
          ),
          goldmark.WithRendererOptions(
              html.WithHardWraps(),
              html.WithXHTML(),
          ),
      )
var buf bytes.Buffer
if err := md.Convert(source, &buf); err != nil {
    panic(err)
}
```

Parser and Renderer options
------------------------------

### Parser options

| Functional option | Type | Description |
| ----------------- | ---- | ----------- |
| `parser.WithBlockParsers` | A `util.PrioritizedSlice` whose elements are `parser.BlockParser` | Parsers for parsing block level elements. |
| `parser.WithInlineParsers` | A `util.PrioritizedSlice` whose elements are `parser.InlineParser` | Parsers for parsing inline level elements. |
| `parser.WithParagraphTransformers` | A `util.PrioritizedSlice` whose elements are `parser.ParagraphTransformer` | Transformers for transforming paragraph nodes. |
| `parser.WithASTTransformers` | A `util.PrioritizedSlice` whose elements are `parser.ASTTransformer` | Transformers for transforming an AST. |
| `parser.WithAutoHeadingID` | `-` | Enables auto heading ids. |
| `parser.WithAttribute` | `-` | Enables custom attributes. Currently only headings supports attributes. |

### HTML Renderer options

| Functional option | Type | Description |
| ----------------- | ---- | ----------- |
| `html.WithWriter` | `html.Writer` | `html.Writer` for writing contents to an `io.Writer`. |
| `html.WithHardWraps` | `-` | Render new lines as `<br>`.|
| `html.WithXHTML` | `-` | Render as XHTML. |
| `html.WithUnsafe` | `-` | By default, goldmark does not render raw HTML and potentially dangerous links. With this option, goldmark renders these contents as written. |

### Built-in extensions

- `extension.Table`
  - [GitHub Flavored Markdown: Tables](https://github.github.com/gfm/#tables-extension-)
- `extension.Strikethrough`
  - [GitHub Flavored Markdown: Strikethrough](https://github.github.com/gfm/#strikethrough-extension-)
- `extension.Linkify`
  - [GitHub Flavored Markdown: Autolinks](https://github.github.com/gfm/#autolinks-extension-)
- `extension.TaskList`
  - [GitHub Flavored Markdown: Task list items](https://github.github.com/gfm/#task-list-items-extension-)
- `extension.GFM`
  - This extension enables Table, Strikethrough, Linkify and TaskList.
  - This extension does not filter tags defined in [6.11: Disallowed Raw HTML (extension)](https://github.github.com/gfm/#disallowed-raw-html-extension-).
    If you need to filter HTML tags, see [Security](#security)
- `extension.DefinitionList`
  - [PHP Markdown Extra: Definition lists](https://michelf.ca/projects/php-markdown/extra/#def-list)
- `extension.Footnote`
  - [PHP Markdown Extra: Footnotes](https://michelf.ca/projects/php-markdown/extra/#footnotes)
- `extension.Typographer`
  - This extension substitutes punctuations with typographic entities like [smartypants](https://daringfireball.net/projects/smartypants/).

### Attributes
`parser.WithAttribute` option allows you to define attributes on some elements.

Currently only headings support attributes.

**Attributes are being discussed in the
[CommonMark forum](https://talk.commonmark.org/t/consistent-attribute-syntax/272).
This syntax may possibly change in the future.**


#### Headings

```
## heading ## {#id .className attrName=attrValue class="class1 class2"}

## heading {#id .className attrName=attrValue class="class1 class2"}
```

```
heading {#id .className attrName=attrValue}
============
```

### Typographer extension

Typographer extension translates plain ASCII punctuation characters into typographic punctuation HTML entities.

Default substitutions are:

| Punctuation | Default entity |
| ------------ | ---------- |
| `'`           | `&lsquo;`, `&rsquo;` |
| `"`           | `&ldquo;`, `&rdquo;` |
| `--`       | `&ndash;` |
| `---`      | `&mdash;` |
| `...`      | `&hellip;` |
| `<<`       | `&laquo;` |
| `>>`       | `&raquo;` |

You can overwrite the substitutions by `extensions.WithTypographicSubstitutions`.

```go
markdown := goldmark.New(
	goldmark.WithExtensions(
		extension.NewTypographer(
			extension.WithTypographicSubstitutions(extension.TypographicSubstitutions{
				extension.LeftSingleQuote:  []byte("&sbquo;"),
				extension.RightSingleQuote: nil, // nil disables a substitution
			}),
		),
	),
)
```

Security
--------------------
By default, goldmark does not render raw HTML and potentially dangerous URLs.
If you need to gain more control over untrusted contents, it is recommended to
use an HTML sanitizer such as [bluemonday](https://github.com/microcosm-cc/bluemonday).

Benchmark
--------------------
You can run this benchmark in the `_benchmark` directory.

### against other golang libraries

blackfriday v2 seems to be fastest, but it is not CommonMark compliant, so the performance of
blackfriday v2 cannot simply be compared with that of the other CommonMark compliant libraries.

Though goldmark builds clean extensible AST structure and get full compliance with
CommonMark, it is reasonably fast and has lower memory consumption.

```
goos: darwin
goarch: amd64
BenchmarkMarkdown/Blackfriday-v2-12                  326           3465240 ns/op         3298861 B/op      20047 allocs/op
BenchmarkMarkdown/GoldMark-12                        303           3927494 ns/op         2574809 B/op      13853 allocs/op
BenchmarkMarkdown/CommonMark-12                      244           4900853 ns/op         2753851 B/op      20527 allocs/op
BenchmarkMarkdown/Lute-12                            130           9195245 ns/op         9175030 B/op     123534 allocs/op
BenchmarkMarkdown/GoMarkdown-12                        9         113541994 ns/op         2187472 B/op      22173 allocs/op
```

### against cmark (CommonMark reference implementation written in C)

```
----------- cmark -----------
file: _data.md
iteration: 50
average: 0.0037760639 sec
go run ./goldmark_benchmark.go
------- goldmark -------
file: _data.md
iteration: 50
average: 0.0040964230 sec
```

As you can see, goldmark performs pretty much equally to cmark.

Extensions
--------------------

- [goldmark-meta](https://github.com/yuin/goldmark-meta): A YAML metadata
  extension for the goldmark Markdown parser.
- [goldmark-highlighting](https://github.com/yuin/goldmark-highlighting): A Syntax highlighting extension
  for the goldmark markdown parser.
- [goldmark-mathjax](https://github.com/litao91/goldmark-mathjax): Mathjax support for goldmark markdown parser

goldmark internal(for extension developers)
----------------------------------------------
### Overview
goldmark's Markdown processing is outlined as a bellow diagram.

```
            <Markdown in []byte, parser.Context>
                           |
                           V
            +-------- parser.Parser ---------------------------
            | 1. Parse block elements into AST
            |   1. If a parsed block is a paragraph, apply 
            |      ast.ParagraphTransformer
            | 2. Traverse AST and parse blocks.
            |   1. Process delimiters(emphasis) at the end of
            |      block parsing
            | 3. Apply parser.ASTTransformers to AST
                           |
                           V
                      <ast.Node>
                           |
                           V
            +------- renderer.Renderer ------------------------
            | 1. Traverse AST and apply renderer.NodeRenderer
            |    corespond to the node type

                           |
                           V
                        <Output>
```

### Parsing
Markdown documents are read through `text.Reader` interface.

AST nodes do not have concrete text. AST nodes have segment information of the documents. It is represented by `text.Segment` .

`text.Segment` has 3 attributes: `Start`, `End`, `Padding` .


**TODO**

See `extension` directory for examples of extensions.

Summary:

1. Define AST Node as a struct in which `ast.BaseBlock` or `ast.BaseInline` is embedded.
2. Write a parser that implements `parser.BlockParser` or `parser.InlineParser`.
3. Write a renderer that implements `renderer.NodeRenderer`.
4. Define your goldmark extension that implements `goldmark.Extender`.


Donation
--------------------
BTC: 1NEDSyUmo4SMTDP83JJQSWi1MvQUGGNMZB

License
--------------------
MIT

Author
--------------------
Yusuke Inuzuka
