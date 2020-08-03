package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-openapi/spec"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/swag"
	"github.com/gorilla/handlers"
	"github.com/toqueteos/webbrowser"
)

// ServeCmd to serve a swagger spec with docs ui
type ServeCmd struct {
	BasePath string `long:"base-path" description:"the base path to serve the spec and UI at"`
	Flavor   string `short:"F" long:"flavor" description:"the flavor of docs, can be swagger or redoc" default:"redoc" choice:"redoc" choice:"swagger"`
	DocURL   string `long:"doc-url" description:"override the url which takes a url query param to render the doc ui"`
	NoOpen   bool   `long:"no-open" description:"when present won't open the the browser to show the url"`
	NoUI     bool   `long:"no-ui" description:"when present, only the swagger spec will be served"`
	Flatten  bool   `long:"flatten" description:"when present, flatten the swagger spec before serving it"`
	Port     int    `long:"port" short:"p" description:"the port to serve this site" env:"PORT"`
	Host     string `long:"host" description:"the interface to serve this site, defaults to 0.0.0.0" env:"HOST"`
}

// Execute the serve command
func (s *ServeCmd) Execute(args []string) error {
	if len(args) == 0 {
		return errors.New("specify the spec to serve as argument to the serve command")
	}

	specDoc, err := loads.Spec(args[0])
	if err != nil {
		return err
	}

	if s.Flatten {
		var err error
		specDoc, err = specDoc.Expanded(&spec.ExpandOptions{
			SkipSchemas:         false,
			ContinueOnError:     true,
			AbsoluteCircularRef: true,
		})

		if err != nil {
			return err
		}
	}

	b, err := json.MarshalIndent(specDoc.Spec(), "", "  ")
	if err != nil {
		return err
	}

	basePath := s.BasePath
	if basePath == "" {
		basePath = "/"
	}

	listener, err := net.Listen("tcp4", net.JoinHostPort(s.Host, strconv.Itoa(s.Port)))
	if err != nil {
		return err
	}
	sh, sp, err := swag.SplitHostPort(listener.Addr().String())
	if err != nil {
		return err
	}
	if sh == "0.0.0.0" {
		sh = "localhost"
	}

	visit := s.DocURL
	handler := http.NotFoundHandler()
	if !s.NoUI {
		if s.Flavor == "redoc" {
			handler = middleware.Redoc(middleware.RedocOpts{
				BasePath: basePath,
				SpecURL:  path.Join(basePath, "swagger.json"),
				Path:     "docs",
			}, handler)
			visit = fmt.Sprintf("http://%s:%d%s", sh, sp, path.Join(basePath, "docs"))
		} else if visit != "" || s.Flavor == "swagger" {
			if visit == "" {
				visit = "http://petstore.swagger.io/"
			}
			u, err := url.Parse(visit)
			if err != nil {
				return err
			}
			q := u.Query()
			q.Add("url", fmt.Sprintf("http://%s:%d%s", sh, sp, path.Join(basePath, "swagger.json")))
			u.RawQuery = q.Encode()
			visit = u.String()
		}
	}

	handler = handlers.CORS()(middleware.Spec(basePath, b, handler))
	errFuture := make(chan error)
	go func() {
		docServer := new(http.Server)
		docServer.SetKeepAlivesEnabled(true)
		docServer.Handler = handler

		errFuture <- docServer.Serve(listener)
	}()

	if !s.NoOpen && !s.NoUI {
		err := webbrowser.Open(visit)
		if err != nil {
			return err
		}
	}
	log.Println("serving docs at", visit)
	return <-errFuture
}
