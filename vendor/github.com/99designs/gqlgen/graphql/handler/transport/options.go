package transport

import (
	"net/http"

	"github.com/99designs/gqlgen/graphql"
)

// Options responds to http OPTIONS and HEAD requests
type Options struct{}

var _ graphql.Transport = Options{}

func (o Options) Supports(r *http.Request) bool {
	return r.Method == "HEAD" || r.Method == "OPTIONS"
}

func (o Options) Do(w http.ResponseWriter, r *http.Request, exec graphql.GraphExecutor) {
	switch r.Method {
	case http.MethodOptions:
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Allow", "OPTIONS, GET, POST")
	case http.MethodHead:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
