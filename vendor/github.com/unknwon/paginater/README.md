# Paginater

[![GitHub Workflow Status](https://img.shields.io/github/workflow/status/unknwon/paginater/Go?logo=github&style=for-the-badge)](https://github.com/unknwon/paginater/actions?query=workflow%3AGo)
[![codecov](https://img.shields.io/codecov/c/github/unknwon/paginater/master?logo=codecov&style=for-the-badge)](https://codecov.io/gh/unknwon/paginater)
[![GoDoc](https://img.shields.io/badge/GoDoc-Reference-blue?style=for-the-badge&logo=go)](https://pkg.go.dev/github.com/unknwon/paginater?tab=doc)
[![Sourcegraph](https://img.shields.io/badge/view%20on-Sourcegraph-brightgreen.svg?style=for-the-badge&logo=sourcegraph)](https://sourcegraph.com/github.com/unknwon/paginater)

Package paginater is a helper module for custom pagination calculation.

## Installation

	go get github.com/Unknwon/paginater

## Getting Started

The following code shows an example of how to use paginater:

```go
package main

import "github.com/Unknwon/paginater"

func main() {
	// Arguments:
	// - Total number of rows
	// - Number of rows in one page
	// - Current page number 
	// - Number of page links to be displayed
	p := paginater.New(45, 10, 3, 3)
	
	// Then use p as a template object named "Page" in "demo.html"
	// ...
}
```

`demo.html`

```html
{{if not .Page.IsFirst}}[First](1){{end}}
{{if .Page.HasPrevious}}[Previous]({{.Page.Previous}}){{end}}

{{range .Page.Pages}}
	{{if eq .Num -1}}
	...
	{{else}}
	{{.Num}}{{if .IsCurrent}}(current){{end}}
	{{end}}
{{end}}

{{if .Page.HasNext}}[Next]({{.Page.Next}}){{end}}
{{if not .Page.IsLast}}[Last]({{.Page.TotalPages}}){{end}}
```

Possible output:

```
[First](1) [Previous](2) ... 2 3(current) 4 ... [Next](4) [Last](5)
```

As you may guess, if the `Page` value is `-1`, you should print `...` in the HTML as common practice.

## Getting Help

- [API Documentation](https://gowalker.org/github.com/Unknwon/paginater)
- [File An Issue](https://github.com/Unknwon/paginater/issues/new)

## License

This project is under Apache v2 License. See the [LICENSE](LICENSE) file for the full license text.