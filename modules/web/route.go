// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/htmlutil"
	"code.gitea.io/gitea/modules/reqctx"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web/middleware"

	"gitea.com/go-chi/binding"
	"github.com/go-chi/chi/v5"
)

// Bind binding an obj to a handler's context data
func Bind[T any](_ T) http.HandlerFunc {
	return func(resp http.ResponseWriter, req *http.Request) {
		theObj := new(T) // create a new form obj for every request but not use obj directly
		data := middleware.GetContextData(req.Context())
		binding.Bind(req, theObj)
		SetForm(data, theObj)
		middleware.AssignForm(theObj, data)
	}
}

// SetForm set the form object
func SetForm(dataStore reqctx.ContextDataProvider, obj any) {
	dataStore.GetData()["__form"] = obj
}

// GetForm returns the validate form information
func GetForm(dataStore reqctx.RequestDataStore) any {
	return dataStore.GetData()["__form"]
}

// Router defines a route based on chi's router
type Router struct {
	chiRouter      chi.Router
	curGroupPrefix string
	curMiddlewares []any
}

// NewRouter creates a new route
func NewRouter() *Router {
	r := chi.NewRouter()
	return &Router{chiRouter: r}
}

// Use supports two middlewares
func (r *Router) Use(middlewares ...any) {
	for _, m := range middlewares {
		if m != nil {
			r.chiRouter.Use(toHandlerProvider(m))
		}
	}
}

// Group mounts a sub-Router along a `pattern` string.
func (r *Router) Group(pattern string, fn func(), middlewares ...any) {
	previousGroupPrefix := r.curGroupPrefix
	previousMiddlewares := r.curMiddlewares
	r.curGroupPrefix += pattern
	r.curMiddlewares = append(r.curMiddlewares, middlewares...)

	fn()

	r.curGroupPrefix = previousGroupPrefix
	r.curMiddlewares = previousMiddlewares
}

func (r *Router) getPattern(pattern string) string {
	newPattern := r.curGroupPrefix + pattern
	if !strings.HasPrefix(newPattern, "/") {
		newPattern = "/" + newPattern
	}
	if newPattern == "/" {
		return newPattern
	}
	return strings.TrimSuffix(newPattern, "/")
}

func isNilOrFuncNil(v any) bool {
	if v == nil {
		return true
	}
	r := reflect.ValueOf(v)
	return r.Kind() == reflect.Func && r.IsNil()
}

func (r *Router) wrapMiddlewareAndHandler(h []any) ([]func(http.Handler) http.Handler, http.HandlerFunc) {
	handlerProviders := make([]func(http.Handler) http.Handler, 0, len(r.curMiddlewares)+len(h)+1)
	for _, m := range r.curMiddlewares {
		if !isNilOrFuncNil(m) {
			handlerProviders = append(handlerProviders, toHandlerProvider(m))
		}
	}
	for _, m := range h {
		if !isNilOrFuncNil(m) {
			handlerProviders = append(handlerProviders, toHandlerProvider(m))
		}
	}
	middlewares := handlerProviders[:len(handlerProviders)-1]
	handlerFunc := handlerProviders[len(handlerProviders)-1](nil).ServeHTTP
	mockPoint := RouterMockPoint(MockAfterMiddlewares)
	if mockPoint != nil {
		middlewares = append(middlewares, mockPoint)
	}
	return middlewares, handlerFunc
}

// Methods adds the same handlers for multiple http "methods" (separated by ",").
// If any method is invalid, the lower level router will panic.
func (r *Router) Methods(methods, pattern string, h ...any) {
	middlewares, handlerFunc := r.wrapMiddlewareAndHandler(h)
	fullPattern := r.getPattern(pattern)
	if strings.Contains(methods, ",") {
		methods := strings.Split(methods, ",")
		for _, method := range methods {
			r.chiRouter.With(middlewares...).Method(strings.TrimSpace(method), fullPattern, handlerFunc)
		}
	} else {
		r.chiRouter.With(middlewares...).Method(methods, fullPattern, handlerFunc)
	}
}

// Mount attaches another Router along ./pattern/*
func (r *Router) Mount(pattern string, subRouter *Router) {
	subRouter.Use(r.curMiddlewares...)
	r.chiRouter.Mount(r.getPattern(pattern), subRouter.chiRouter)
}

// Any delegate requests for all methods
func (r *Router) Any(pattern string, h ...any) {
	middlewares, handlerFunc := r.wrapMiddlewareAndHandler(h)
	r.chiRouter.With(middlewares...).HandleFunc(r.getPattern(pattern), handlerFunc)
}

// Delete delegate delete method
func (r *Router) Delete(pattern string, h ...any) {
	r.Methods("DELETE", pattern, h...)
}

// Get delegate get method
func (r *Router) Get(pattern string, h ...any) {
	r.Methods("GET", pattern, h...)
}

// Head delegate head method
func (r *Router) Head(pattern string, h ...any) {
	r.Methods("HEAD", pattern, h...)
}

// Post delegate post method
func (r *Router) Post(pattern string, h ...any) {
	r.Methods("POST", pattern, h...)
}

// Put delegate put method
func (r *Router) Put(pattern string, h ...any) {
	r.Methods("PUT", pattern, h...)
}

// Patch delegate patch method
func (r *Router) Patch(pattern string, h ...any) {
	r.Methods("PATCH", pattern, h...)
}

// ServeHTTP implements http.Handler
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.normalizeRequestPath(w, req, r.chiRouter)
}

// NotFound defines a handler to respond whenever a route could not be found.
func (r *Router) NotFound(h http.HandlerFunc) {
	r.chiRouter.NotFound(h)
}

type pathProcessorParam struct {
	name         string
	captureGroup int
}

type PathProcessor struct {
	methods container.Set[string]
	re      *regexp.Regexp
	params  []pathProcessorParam
}

func (r *Router) normalizeRequestPath(resp http.ResponseWriter, req *http.Request, next http.Handler) {
	normalized := false
	normalizedPath := req.URL.EscapedPath()
	if normalizedPath == "" {
		normalizedPath, normalized = "/", true
	} else if normalizedPath != "/" {
		normalized = strings.HasSuffix(normalizedPath, "/")
		normalizedPath = strings.TrimRight(normalizedPath, "/")
	}
	removeRepeatedSlashes := strings.Contains(normalizedPath, "//")
	normalized = normalized || removeRepeatedSlashes

	// the following code block is a slow-path for replacing all repeated slashes "//" to one single "/"
	// if the path doesn't have repeated slashes, then no need to execute it
	if removeRepeatedSlashes {
		buf := &strings.Builder{}
		for i := 0; i < len(normalizedPath); i++ {
			if i == 0 || normalizedPath[i-1] != '/' || normalizedPath[i] != '/' {
				buf.WriteByte(normalizedPath[i])
			}
		}
		normalizedPath = buf.String()
	}

	// If the config tells Gitea to use a sub-url path directly without reverse proxy,
	// then we need to remove the sub-url path from the request URL path.
	// But "/v2" is special for OCI container registry, it should always be in the root of the site.
	if setting.UseSubURLPath {
		remainingPath, ok := strings.CutPrefix(normalizedPath, setting.AppSubURL+"/")
		if ok {
			normalizedPath = "/" + remainingPath
		} else if normalizedPath == setting.AppSubURL {
			normalizedPath = "/"
		} else if !strings.HasPrefix(normalizedPath+"/", "/v2/") {
			// do not respond to other requests, to simulate a real sub-path environment
			resp.Header().Add("Content-Type", "text/html; charset=utf-8")
			resp.WriteHeader(http.StatusNotFound)
			_, _ = resp.Write([]byte(htmlutil.HTMLFormat(`404 page not found, sub-path is: <a href="%s">%s</a>`, setting.AppSubURL, setting.AppSubURL)))
			return
		}
		normalized = true
	}

	// if the path is normalized, then fill it back to the request
	if normalized {
		decodedPath, err := url.PathUnescape(normalizedPath)
		if err != nil {
			http.Error(resp, "400 Bad Request: unable to unescape path "+normalizedPath, http.StatusBadRequest)
			return
		}
		req.URL.RawPath = normalizedPath
		req.URL.Path = decodedPath
	}

	next.ServeHTTP(resp, req)
}

func (p *PathProcessor) ProcessRequestPath(chiCtx *chi.Context, path string) bool {
	if !p.methods.Contains(chiCtx.RouteMethod) {
		return false
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	pathMatches := p.re.FindStringSubmatchIndex(path) // Golang regexp match pairs [start, end, start, end, ...]
	if pathMatches == nil {
		return false
	}
	var paramMatches [][]int
	for i := 2; i < len(pathMatches); {
		paramMatches = append(paramMatches, []int{pathMatches[i], pathMatches[i+1]})
		pmIdx := len(paramMatches) - 1
		end := pathMatches[i+1]
		i += 2
		for ; i < len(pathMatches); i += 2 {
			if pathMatches[i] >= end {
				break
			}
			paramMatches[pmIdx] = append(paramMatches[pmIdx], pathMatches[i], pathMatches[i+1])
		}
	}
	for i, pm := range paramMatches {
		groupIdx := p.params[i].captureGroup * 2
		chiCtx.URLParams.Add(p.params[i].name, path[pm[groupIdx]:pm[groupIdx+1]])
	}
	return true
}

func NewPathProcessor(methods, pattern string) *PathProcessor {
	p := &PathProcessor{methods: make(container.Set[string])}
	for _, method := range strings.Split(methods, ",") {
		p.methods.Add(strings.TrimSpace(method))
	}
	re := []byte{'^'}
	lastEnd := 0
	for lastEnd < len(pattern) {
		start := strings.IndexByte(pattern[lastEnd:], '<')
		if start == -1 {
			re = append(re, pattern[lastEnd:]...)
			break
		}
		end := strings.IndexByte(pattern[lastEnd+start:], '>')
		if end == -1 {
			panic(fmt.Sprintf("invalid pattern: %s", pattern))
		}
		re = append(re, pattern[lastEnd:lastEnd+start]...)
		partName, partExp, _ := strings.Cut(pattern[lastEnd+start+1:lastEnd+start+end], ":")
		lastEnd += start + end + 1

		// TODO: it could support to specify a "capture group" for the name, for example: "/<name[2]:(\d)-(\d)>"
		// it is not used so no need to implement it now
		param := pathProcessorParam{}
		if partExp == "*" {
			re = append(re, "(.*?)/?"...)
			if lastEnd < len(pattern) {
				if pattern[lastEnd] == '/' {
					lastEnd++
				}
			}
		} else {
			partExp = util.IfZero(partExp, "[^/]+")
			re = append(re, '(')
			re = append(re, partExp...)
			re = append(re, ')')
		}
		param.name = partName
		p.params = append(p.params, param)
	}
	re = append(re, '$')
	reStr := string(re)
	p.re = regexp.MustCompile(reStr)
	return p
}

// Combo delegates requests to Combo
func (r *Router) Combo(pattern string, h ...any) *Combo {
	return &Combo{r, pattern, h}
}

// Combo represents a tiny group routes with same pattern
type Combo struct {
	r       *Router
	pattern string
	h       []any
}

// Get delegates Get method
func (c *Combo) Get(h ...any) *Combo {
	c.r.Get(c.pattern, append(c.h, h...)...)
	return c
}

// Post delegates Post method
func (c *Combo) Post(h ...any) *Combo {
	c.r.Post(c.pattern, append(c.h, h...)...)
	return c
}

// Delete delegates Delete method
func (c *Combo) Delete(h ...any) *Combo {
	c.r.Delete(c.pattern, append(c.h, h...)...)
	return c
}

// Put delegates Put method
func (c *Combo) Put(h ...any) *Combo {
	c.r.Put(c.pattern, append(c.h, h...)...)
	return c
}

// Patch delegates Patch method
func (c *Combo) Patch(h ...any) *Combo {
	c.r.Patch(c.pattern, append(c.h, h...)...)
	return c
}
