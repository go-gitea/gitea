go-version [![Build Status](https://travis-ci.org/mcuadros/go-version.png?branch=master)](https://travis-ci.org/mcuadros/go-version) [![GoDoc](https://godoc.org/github.com/mcuadros/go-version?status.png)](http://godoc.org/github.com/mcuadros/go-version)
==============================

Version normalizer and comparison library for go, heavy based on PHP version_compare function and Version comparsion libs from [Composer](https://github.com/composer/composer) PHP project

Installation
------------

The recommended way to install go-version

```
go get github.com/mcuadros/go-version
```

Examples
--------

How import the package

```go
import (
    "github.com/mcuadros/go-version"
)
```

`version.Normalize()`: Normalizes a version string to be able to perform comparisons on it

```go
version.Normalize("10.4.13-b")
//Returns: 10.4.13.0-beta
```


`version.CompareSimple()`: Compares two normalizated version number strings

```go
version.CompareSimple("1.2", "1.0.1")
//Returns: 1

version.CompareSimple("1.0rc1", "1.0")
//Returns: -1
```


`version.Compare()`: Compares two normalizated version number strings, for a particular relationship

```go
version.Compare("1.0-dev", "1.0", "<")
//Returns: true

version.Compare("1.0rc1", "1.0", ">=")
//Returns: false

version.Compare("2.3.4", "v3.1.2", "<")
//Returns: true
```

`version.ConstrainGroup.Match()`: Match a given version againts a group of constrains, read about constraint string format at [Composer documentation](http://getcomposer.org/doc/01-basic-usage.md#package-versions)  

```go
c := version.NewConstrainGroupFromString(">2.0,<=3.0")
c.Match("2.5.0beta")
//Returns: true

c := version.NewConstrainGroupFromString("~1.2.3")
c.Match("1.2.3.5")
//Returns: true
```

`version.Sort()`: Sorts a string slice of version number strings using version.CompareSimple()

```go
version.Sort([]string{"1.10-dev", "1.0rc1", "1.0", "1.0-dev"})
//Returns []string{"1.0-dev", "1.0rc1", "1.0", "1.10-dev"}
```

License
-------

MIT, see [LICENSE](LICENSE)

[![Bitdeli Badge](https://d2weczhvl823v0.cloudfront.net/mcuadros/go-version/trend.png)](https://bitdeli.com/free "Bitdeli Badge")
