# Collection of compression libraries for Go #

[![GoDoc](https://godoc.org/github.com/dsnet/compress/cmp?status.svg)](https://godoc.org/github.com/dsnet/compress)
[![Build Status](https://travis-ci.org/dsnet/compress.svg?branch=master)](https://travis-ci.org/dsnet/compress)
[![Report Card](https://goreportcard.com/badge/github.com/dsnet/compress)](https://goreportcard.com/report/github.com/dsnet/compress)

## Introduction ##

**NOTE: This library is in active development. As such, there are no guarantees about the stability of the API. The author reserves the right to arbitrarily break the API for any reason.**

This repository hosts a collection of compression related libraries. The goal of this project is to provide pure Go implementations for popular compression algorithms beyond what the Go standard library provides. The goals for these packages are as follows:
* Maintainable: That the code remains well documented, well tested, readable, easy to maintain, and easy to verify that it conforms to the specification for the format being implemented.
* Performant: To be able to compress and decompress within at least 80% of the rates that the C implementations are able to achieve.
* Flexible: That the code provides low-level and fine granularity control over the compression streams similar to what the C APIs would provide.

Of these three, the first objective is often at odds with the other two objectives and provides interesting challenges. Higher performance can often be achieved by muddling abstraction layers or using non-intuitive low-level primitives. Also, more features and functionality, while useful in some situations, often complicates the API. Thus, this package will attempt to satisfy all the goals, but will defer to favoring maintainability when the performance or flexibility benefits are not significant enough.


## Library Status ##

For the packages available, only some features are currently implemented:

| Package | Reader | Writer |
| ------- | :----: | :----: |
| brotli | :white_check_mark: | |
| bzip2 | :white_check_mark: | :white_check_mark: |
| flate | :white_check_mark: | |
| xflate | :white_check_mark: | :white_check_mark: |

This library is in active development. As such, there are no guarantees about the stability of the API. The author reserves the right to arbitrarily break the API for any reason. When the library becomes more mature, it is planned to eventually conform to some strict versioning scheme like [Semantic Versioning](http://semver.org/).

However, in the meanwhile, this library does provide some basic API guarantees. For the types defined below, the method signatures are guaranteed to not change. Note that the author still reserves the right to change the fields within each ```Reader``` and ```Writer``` structs.
```go
type ReaderConfig struct { ... }
type Reader struct { ... }
  func NewReader(io.Reader, *ReaderConfig) (*Reader, error) { ... }
  func (*Reader) Read([]byte) (int, error)                  { ... }
  func (*Reader) Close() error                              { ... }

type WriterConfig struct { ... }
type Writer struct { ... }
  func NewWriter(io.Writer, *WriterConfig) (*Writer, error) { ... }
  func (*Writer) Write([]byte) (int, error)                 { ... }
  func (*Writer) Close() error                              { ... }
```

To see what work still remains, see the [Task List](https://github.com/dsnet/compress/wiki/Task-List).

## Performance  ##

See [Performance Metrics](https://github.com/dsnet/compress/wiki/Performance-Metrics).


## Frequently Asked Questions ##

See [Frequently Asked Questions](https://github.com/dsnet/compress/wiki/Frequently-Asked-Questions).


## Installation ##

Run the command:

```go get -u github.com/dsnet/compress```

This library requires `Go1.9` or higher in order to build.


## Packages ##

| Package | Description |
| :------ | :---------- |
| [brotli](http://godoc.org/github.com/dsnet/compress/brotli) | Package brotli implements the Brotli format, described in RFC 7932. |
| [bzip2](http://godoc.org/github.com/dsnet/compress/bzip2) | Package bzip2 implements the BZip2 compressed data format. |
| [flate](http://godoc.org/github.com/dsnet/compress/flate) | Package flate implements the DEFLATE format, described in RFC 1951. |
| [xflate](http://godoc.org/github.com/dsnet/compress/xflate) | Package xflate implements the XFLATE format, an random-access extension to DEFLATE. |
