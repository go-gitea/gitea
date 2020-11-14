Package renderer
==================
[![Build Status](https://travis-ci.org/thedevsaddam/renderer.svg?branch=master)](https://travis-ci.org/thedevsaddam/renderer)
[![Project status](https://img.shields.io/badge/version-1.2-green.svg)](https://github.com/thedevsaddam/renderer/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/thedevsaddam/renderer)](https://goreportcard.com/report/github.com/thedevsaddam/renderer)
[![Coverage Status](https://coveralls.io/repos/github/thedevsaddam/renderer/badge.svg?branch=master)](https://coveralls.io/github/thedevsaddam/renderer?branch=master)
[![GoDoc](https://godoc.org/github.com/thedevsaddam/renderer?status.svg)](https://godoc.org/github.com/thedevsaddam/renderer)
[![License](https://img.shields.io/dub/l/vibe-d.svg)](https://github.com/thedevsaddam/renderer/blob/dev/LICENSE.md)

Simple, lightweight and faster response (JSON, JSONP, XML, YAML, HTML, File) rendering package for Go

### Installation

Install the package using
```go
$ go get github.com/thedevsaddam/renderer/...
```

### Usage

To use the package import it in your `*.go` code
```go
import "github.com/thedevsaddam/renderer"
```
### Example

```go
package main

import (
	"io"
	"log"
	"net/http"
	"os"

	"github.com/thedevsaddam/renderer"
)

func main() {
	rnd := renderer.New()

	mux := http.NewServeMux()

	usr := struct {
		Name string
		Age  int
	}{"John Doe", 30}

	// serving String as text/plain
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		rnd.String(w, http.StatusOK, "Welcome to renderer")
	})

	// serving success but no content
	mux.HandleFunc("/no-content", func(w http.ResponseWriter, r *http.Request) {
		rnd.NoContent(w)
	})

	// serving string as html
	mux.HandleFunc("/html-string", func(w http.ResponseWriter, r *http.Request) {
		rnd.HTMLString(w, http.StatusOK, "<h1>Hello Renderer!</h1>")
	})

	// serving JSON
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		rnd.JSON(w, http.StatusOK, usr)
	})

	// serving JSONP
	mux.HandleFunc("/jsonp", func(w http.ResponseWriter, r *http.Request) {
		rnd.JSONP(w, http.StatusOK, "callback", usr)
	})

	// serving XML
	mux.HandleFunc("/xml", func(w http.ResponseWriter, r *http.Request) {
		rnd.XML(w, http.StatusOK, usr)
	})

	// serving YAML
	mux.HandleFunc("/yaml", func(w http.ResponseWriter, r *http.Request) {
		rnd.YAML(w, http.StatusOK, usr)
	})

	// serving File as arbitary binary data
	mux.HandleFunc("/binary", func(w http.ResponseWriter, r *http.Request) {
		var reader io.Reader
		reader, _ = os.Open("../README.md")
		rnd.Binary(w, http.StatusOK, reader, "readme.md", true)
	})

	// serving File as inline
	mux.HandleFunc("/file-inline", func(w http.ResponseWriter, r *http.Request) {
		rnd.FileView(w, http.StatusOK, "../README.md", "readme.md")
	})

	// serving File as attachment
	mux.HandleFunc("/file-download", func(w http.ResponseWriter, r *http.Request) {
		rnd.FileDownload(w, http.StatusOK, "../README.md", "readme.md")
	})

	// serving File from reader as inline
	mux.HandleFunc("/file-reader", func(w http.ResponseWriter, r *http.Request) {
		var reader io.Reader
		reader, _ = os.Open("../README.md")
		rnd.File(w, http.StatusOK, reader, "readme.md", true)
	})

	// serving custom response using render and chaining methods
	mux.HandleFunc("/render", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(renderer.ContentType, renderer.ContentText)
		rnd.Render(w, http.StatusOK, []byte("Send the message as text response"))
	})

	port := ":9000"
	log.Println("Listening on port", port)
	http.ListenAndServe(port, mux)
}

```

### How to render html template?

Well, you can parse html template using `HTML`, `View`, `Template` any of these method. These are based on `html/template` package.

When using `Template` method you can simply pass the base layouts, templates path as a slice of string.

***Template example***

You can parse template on the fly using `Template` method. You can set delimiter, inject FuncMap easily.

template/layout.tmpl
```html
<html>
  <head>
    <title>{{ template "title" . }}</title>
  </head>
  <body>
    {{ template "content" . }}
  </body>
  {{ block "sidebar" .}}{{end}}
</html>
```
template/index.tmpl
```html
{{ define "title" }}Home{{ end }}

{{ define "content" }}
  <h1>Hello, {{ .Name | toUpper }}</h1>
  <p>Lorem ipsum dolor sit amet, consectetur adipisicing elit.</p>
{{ end }}
```
template/partial.tmpl
```html
{{define "sidebar"}}
  simple sidebar code
{{end}}

```

template.go
```go
package main

import (
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/thedevsaddam/renderer"
)

var rnd *renderer.Render

func init() {
	rnd = renderer.New()
}

func toUpper(s string) string {
	return strings.ToUpper(s)
}

func handler(w http.ResponseWriter, r *http.Request) {
	usr := struct {
		Name string
		Age  int
	}{"john doe", 30}

	tpls := []string{"template/layout.tmpl", "template/index.tmpl", "template/partial.tmpl"}
	rnd.FuncMap(template.FuncMap{
		"toUpper": toUpper,
	})
	err := rnd.Template(w, http.StatusOK, tpls, usr)
	if err != nil {
		log.Fatal(err) //respond with error page or message
	}
}

func main() {
	http.HandleFunc("/", handler)
	log.Println("Listening port: 9000")
	http.ListenAndServe(":9000", nil)
}
```

***HTML example***

When using `HTML` you can parse a template directory using `pattern` and call the template by their name. See the example code below:

html/index.html
```html
{{define "indexPage"}}
    <html>
    {{template "header"}}
    <body>
        <h3>Index</h3>
        <p>Lorem ipsum dolor sit amet, consectetur adipisicing elit</p>
    </body>
    {{template "footer"}}
    </html>
{{end}}
```

html/header.html
```html
{{define "header"}}
     <head>
         <title>Header</title>
         <h1>Header section</h1>
     </head>
{{end}}
```

html/footer.html
```html
{{define "footer"}}
     <footer>Copyright &copy; 2020</footer>
{{end}}
```

html.go
```go
package main

import (
	"log"
	"net/http"

	"github.com/thedevsaddam/renderer"
)

var rnd *renderer.Render

func init() {
	rnd = renderer.New(
		renderer.Options{
			ParseGlobPattern: "html/*.html",
		},
	)
}

func handler(w http.ResponseWriter, r *http.Request) {
	err := rnd.HTML(w, http.StatusOK, "indexPage", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	http.HandleFunc("/", handler)
	log.Println("Listening port: 9000")
	http.ListenAndServe(":9000", nil)
}
```

***View example***

When using `View` for parsing template you can pass multiple layout and templates. Here template name will be the file name. See the example to get the idea.

view/base.lout
```html
<html>
  <head>
     <title>{{block "title" .}} {{end}}</title>
  </head>
  <body>
    {{ template "content" . }}
  </body>
</html>
```

view/home.tpl
```html
{{define "title"}}Home{{end}}
{{define "content"}}
<h3>Home page</h3>
    <ul>
        <li><a href="/">Home</a></li>
        <li><a href="/about">About Me</a></li>
    </ul>
    <p>
    Lorem ipsum dolor sit amet</p>
{{end}}
```
view/about.tpl
```html
{{define "title"}}About Me{{end}}
{{define "content"}}
<h2>This is About me page.</h2>
<ul>
    Lorem ipsum dolor sit amet, consectetur adipisicing elit,
</ul>
<p><a href="/">Home</a></p>
{{end}}
```

view.go
```go
package main

import (
	"log"
	"net/http"

	"github.com/thedevsaddam/renderer"
)

var rnd *renderer.Render

func init() {
	rnd = renderer.New(renderer.Options{
		TemplateDir: "view",
	})
}

func home(w http.ResponseWriter, r *http.Request) {
	err := rnd.View(w, http.StatusOK, "home", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func about(w http.ResponseWriter, r *http.Request) {
	err := rnd.View(w, http.StatusOK, "about", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	http.HandleFunc("/", home)
	http.HandleFunc("/about", about)
	log.Println("Listening port: 9000\n / is root \n /about is about page")
	http.ListenAndServe(":9000", nil)
}
```

***Note:*** This is a wrapper on top of go built-in packages to provide syntactic sugar.

### Contribution
Your suggestions will be more than appreciated.
[Read the contribution guide here](CONTRIBUTING.md)

### See all [contributors](https://github.com/thedevsaddam/renderer/graphs/contributors)

### Read [API doc](https://godoc.org/github.com/thedevsaddam/renderer) to know about ***Available options and Methods***

### **License**
The **renderer** is an open-source software licensed under the [MIT License](LICENSE.md).
