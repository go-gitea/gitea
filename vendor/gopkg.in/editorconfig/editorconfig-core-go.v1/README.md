[![GoDoc](https://godoc.org/gopkg.in/editorconfig/editorconfig-core-go.v1?status.svg)](https://godoc.org/gopkg.in/editorconfig/editorconfig-core-go.v1)
[![Go Report Card](https://goreportcard.com/badge/gopkg.in/editorconfig/editorconfig-core-go.v1)](https://goreportcard.com/report/gopkg.in/editorconfig/editorconfig-core-go.v1)

# Editorconfig Core Go

A [Editorconfig][editorconfig] file parser and manipulator for Go.

> This package is already working, but still under testing.

## Installing

We recommend the use of [gopkg.in][gopkg] for this package:

```bash
go get -u gopkg.in/editorconfig/editorconfig-core-go.v1
```

Import by the same path. Tha package name you will use to access it is
`editorconfig`.

```go
import (
    "gopkg.in/editorconfig/editorconfig-core-go.v1"
)
```

## Usage

### Parse from file

```go
editorConfig, err := editorconfig.ParseFile("path/to/.editorconfig")
if err != nil {
    log.Fatal(err)
}
```

### Parse from slice of bytes

```go
data := []byte("...")
editorConfig, err := editorconfig.ParseBytes(data)
if err != nil {
    log.Fatal(err)
}
```

### Get definition to a given filename

This method builds a definition to a given filename.
This definition is a merge of the properties with selectors that matched the
given filename.
The lasts sections of the file have preference over the priors.

```go
def := editorConfig.GetDefinitionForFilename("my/file.go")
```

This definition have the following properties:

```go
type Definition struct {
	Selector string

	Charset                string
	IndentStyle            string
	IndentSize             string
	TabWidth               int
	EndOfLine              string
	TrimTrailingWhitespace bool
	InsertFinalNewline     bool
}
```

#### Automatic search for `.editorconfig` files

If you want a definition of a file without having to manually
parse the `.editorconfig` files, you can then use the static version
of `GetDefinitionForFilename`:

```go
def, err := editorconfig.GetDefinitionForFilename("foo/bar/baz/my-file.go")
```

In the example above, the package will automatically search for
`.editorconfig` files on:

- `foo/bar/baz/.editorconfig`
- `foo/baz/.editorconfig`
- `foo/.editorconfig`

Until it reaches a file with `root = true` or the root of the filesystem.

### Generating a .editorconfig file

You can easily convert a Editorconfig struct to a compatible INI file:

```go
// serialize to slice of bytes
data, err := editorConfig.Serialize()
if err != nil {
    log.Fatal(err)
}

// save directly to file
err := editorConfig.Save("path/to/.editorconfig")
if err != nil {
    log.Fatal(err)
}
```

## Contributing

To run the tests:

```bash
go test -v
```

[editorconfig]: http://editorconfig.org/
[gopkg]: https://gopkg.in
