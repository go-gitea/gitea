package middleware

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"path"
)

// RapiDocOpts configures the RapiDoc middlewares
type RapiDocOpts struct {
	// BasePath for the UI path, defaults to: /
	BasePath string
	// Path combines with BasePath for the full UI path, defaults to: docs
	Path string
	// SpecURL the url to find the spec for
	SpecURL string
	// RapiDocURL for the js that generates the rapidoc site, defaults to: https://cdn.jsdelivr.net/npm/rapidoc/bundles/rapidoc.standalone.js
	RapiDocURL string
	// Title for the documentation site, default to: API documentation
	Title string
}

// EnsureDefaults in case some options are missing
func (r *RapiDocOpts) EnsureDefaults() {
	if r.BasePath == "" {
		r.BasePath = "/"
	}
	if r.Path == "" {
		r.Path = "docs"
	}
	if r.SpecURL == "" {
		r.SpecURL = "/swagger.json"
	}
	if r.RapiDocURL == "" {
		r.RapiDocURL = rapidocLatest
	}
	if r.Title == "" {
		r.Title = "API documentation"
	}
}

// RapiDoc creates a middleware to serve a documentation site for a swagger spec.
// This allows for altering the spec before starting the http listener.
//
func RapiDoc(opts RapiDocOpts, next http.Handler) http.Handler {
	opts.EnsureDefaults()

	pth := path.Join(opts.BasePath, opts.Path)
	tmpl := template.Must(template.New("rapidoc").Parse(rapidocTemplate))

	buf := bytes.NewBuffer(nil)
	_ = tmpl.Execute(buf, opts)
	b := buf.Bytes()

	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pth {
			rw.Header().Set("Content-Type", "text/html; charset=utf-8")
			rw.WriteHeader(http.StatusOK)

			_, _ = rw.Write(b)
			return
		}

		if next == nil {
			rw.Header().Set("Content-Type", "text/plain")
			rw.WriteHeader(http.StatusNotFound)
			_, _ = rw.Write([]byte(fmt.Sprintf("%q not found", pth)))
			return
		}
		next.ServeHTTP(rw, r)
	})
}

const (
	rapidocLatest   = "https://unpkg.com/rapidoc/dist/rapidoc-min.js"
	rapidocTemplate = `<!doctype html>
<html>
<head>
  <title>{{ .Title }}</title>
  <meta charset="utf-8"> <!-- Important: rapi-doc uses utf8 charecters -->
  <script type="module" src="{{ .RapiDocURL }}"></script>
</head>
<body>
  <rapi-doc spec-url="{{ .SpecURL }}"></rapi-doc>
</body>
</html>
`
)
