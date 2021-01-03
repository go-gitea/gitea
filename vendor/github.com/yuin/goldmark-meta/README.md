goldmark-meta
=========================
[![GoDev][godev-image]][godev-url]

[godev-image]: https://pkg.go.dev/badge/github.com/yuin/goldmark-meta
[godev-url]: https://pkg.go.dev/github.com/yuin/goldmark-meta


goldmark-meta is an extension for the [goldmark](http://github.com/yuin/goldmark) 
that allows you to define document metadata in YAML format.

Usage
--------------------

### Installation

```
go get github.com/yuin/goldmark-meta
```

### Markdown syntax

YAML metadata block is a leaf block that can not have any markdown element
as a child.

YAML metadata must start with a **YAML metadata separator**.
This separator must be at first line of the document.

A **YAML metadata separator** is a line that only `-` is repeated.

YAML metadata must end with a **YAML metadata separator**.

You can define objects as a 1st level item. At deeper level, you can define 
any kind of YAML element.

Example:

```
---
Title: goldmark-meta
Summary: Add YAML metadata to the document
Tags:
    - markdown
    - goldmark
---

# Heading 1
```


### Access the metadata

```go
import (
    "bytes"
    "fmt"
    "github.com/yuin/goldmark"
    "github.com/yuin/goldmark/extension"
    "github.com/yuin/goldmark/parser"
    "github.com/yuin/goldmark-meta"
)

func main() {
    markdown := goldmark.New(
        goldmark.WithExtensions(
            meta.Meta,
        ),
    )
    source := `---
Title: goldmark-meta
Summary: Add YAML metadata to the document
Tags:
    - markdown
    - goldmark
---

# Hello goldmark-meta
`

    var buf bytes.Buffer
    context := parser.NewContext()
    if err := markdown.Convert([]byte(source), &buf, parser.WithContext(context)); err != nil {
        panic(err)
    }
    metaData := meta.Get(context)
    title := metaData["Title"]
    fmt.Print(title)
}
```

### Render the metadata as a table

You need to add `extension.TableHTMLRenderer` or the `Table` extension to
render metadata as a table.

```go
import (
    "bytes"
    "fmt"
    "github.com/yuin/goldmark"
    "github.com/yuin/goldmark/extension"
    "github.com/yuin/goldmark/parser"
    "github.com/yuin/goldmark/renderer"
    "github.com/yuin/goldmark/util"
    "github.com/yuin/goldmark-meta"
)

func main() {
    markdown := goldmark.New(
        goldmark.WithExtensions(
            meta.New(meta.WithTable()),
        ),
        goldmark.WithRendererOptions(
            renderer.WithNodeRenderers(
                util.Prioritized(extension.NewTableHTMLRenderer(), 500),
            ),
        ),
    )
    // OR
    // markdown := goldmark.New(
    //     goldmark.WithExtensions(
    //         meta.New(meta.WithTable()),
    //         extension.Table,
    //     ),
    // )
    source := `---
Title: goldmark-meta
Summary: Add YAML metadata to the document
Tags:
    - markdown
    - goldmark
---

# Hello goldmark-meta
`

    var buf bytes.Buffer
    if err := markdown.Convert([]byte(source), &buf); err != nil {
        panic(err)
    }
    fmt.Print(buf.String())
}
```


License
--------------------
MIT

Author
--------------------
Yusuke Inuzuka
