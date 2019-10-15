<!-- Currently tests against core-test are not done so hide build status badge for now -->
<!-- [![Build Status](https://travis-ci.org/editorconfig/editorconfig-core-go.svg?branch=master)](https://travis-ci.org/editorconfig/editorconfig-core-go) -->
[![GoDoc](https://godoc.org/github.com/editorconfig/editorconfig-core-go?status.svg)](https://godoc.org/github.com/editorconfig/editorconfig-core-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/editorconfig/editorconfig-core-go)](https://goreportcard.com/report/github.com/editorconfig/editorconfig-core-go)

# Editorconfig Core Go

A [Editorconfig][editorconfig] file parser and manipulator for Go.

> Currently this package does some basic work but does not fully support
> EditorConfig specs, so using it in "real world" is not recommended.

## Missing features

- `unset`
- escaping comments in values, probably in [go-ini/ini](https://github.com/go-ini/ini)

## Installing

We recommend the use of Go 1.11+ modules for this package.

Import by the same path. The package name you will use to access it is
`editorconfig`.

```go
import (
    "github.com/editorconfig/editorconfig-core-go/v2"
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
	Raw                    map[string]string
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
go test -v ./...
```

[editorconfig]: http://editorconfig.org/
