![Build Status](https://github.com/editorconfig/editorconfig-core-go/workflows/.github/workflows/main.yml/badge.svg)
[![GoDoc](https://godoc.org/github.com/editorconfig/editorconfig-core-go?status.svg)](https://godoc.org/github.com/editorconfig/editorconfig-core-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/editorconfig/editorconfig-core-go)](https://goreportcard.com/report/github.com/editorconfig/editorconfig-core-go)

# Editorconfig Core Go

A [Editorconfig][editorconfig] file parser and manipulator for Go.

## Missing features

- escaping comments in values, probably in [go-ini/ini](https://github.com/go-ini/ini)
- [adjacent nested braces](https://github.com/editorconfig/editorconfig-core-test/pull/44)

## Installing

We recommend the use of Go 1.13+ modules for this package.

Import by the same path. The package name you will use to access it is
`editorconfig`.

```go
import "github.com/editorconfig/editorconfig-core-go/v2"
```

## Usage

### Parse from file

```go
fp, err := os.Open("path/to/.editorconfig")
if err != nil {
	log.Fatal(err)
}
defer fp.Close()

editorConfig, err := editorconfig.Parse(fp)
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
	TrimTrailingWhitespace *bool
	InsertFinalNewline     *bool
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

To run the [integration tests](https://github.com/editorconfig/editorconfig-core-test):

```
make test-core
```

[editorconfig]: https://editorconfig.org/
