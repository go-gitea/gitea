goldmark-highlighting
=========================

goldmark-highlighting is an extension for the [goldmark](http://github.com/yuin/goldmark) 
that adds syntax-highlighting to the fenced code blocks.

goldmark-highlighting uses [chroma](https://github.com/alecthomas/chroma) as a
syntax highlighter.

Installation
--------------------

```
go get github.com/yuin/goldmark-highlighting
```

Usage
--------------------

```go
import (
    "bytes"
    "fmt"
    "github.com/alecthomas/chroma/formatters/html"
    "github.com/yuin/goldmark"
    "github.com/yuin/goldmark/extension"
    "github.com/yuin/goldmark/parser"
    "github.com/yuin/goldmark-highlighting"

)

func main() {
    markdown := goldmark.New(
        goldmark.WithExtensions(
            highlighting.Highlighting,
        ),
    )
    var buf bytes.Buffer
    if err := markdown.Convert([]byte(source), &buf); err != nil {
        panic(err)
    }
    fmt.Print(title)
}
```


```go
    markdown := goldmark.New(
        goldmark.WithExtensions(
            highlighting.NewHighlighting(
               highlighting.WithStyle("monokai"),
               highlighting.WithFormatOptions(
                   html.WithLineNumbers(),
               ),
            ),
        ),
    )
```

License
--------------------
MIT

Author
--------------------
Yusuke Inuzuka
