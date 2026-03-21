// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"code.gitea.io/gitea/modules/htmlutil"
	"code.gitea.io/gitea/modules/reqctx"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web/middleware"

	"gitea.com/go-chi/binding"
	"github.com/go-chi/chi/v5"
)

// PreMiddlewareProvider is a special middleware provider which will be executed
// before other middlewares on the same "routing" level (AfterRouting/Group/Methods/Any, but not BeforeRouting).
// A route can do something (e.g.: set middleware options) at the place where it is declared,
// and the code will be executed before other middlewares which are added before the declaration.
// Use cases: mark a route with some meta info, set some options for middlewares, etc.
type PreMiddlewareProvider func(next http.Handler) http.Handler

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
	chiRouter *chi.Mux

	afterRouting []any

	curGroupPrefix string
	curMiddlewares []any
}

// NewRouter creates a new route
func NewRouter() *Router {
	r := chi.NewRouter()
	return &Router{chiRouter: r}
}

// BeforeRouting adds middlewares which will be executed before the request path gets routed
// It should only be used for framework-level global middlewares when it needs to change request method & path.
func (r *Router) BeforeRouting(middlewares ...any) {
	for _, m := range middlewares {
		if !isNilOrFuncNil(m) {
			r.chiRouter.Use(toHandlerProvider(m))
		}
	}
}

// AfterRouting adds middlewares which will be executed after the request path gets routed
// It can see the routed path and resolved path parameters
func (r *Router) AfterRouting(middlewares ...any) {
	r.afterRouting = append(r.afterRouting, middlewares...)
}

// Group mounts a sub-router along a "pattern" string.
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

func wrapMiddlewareAppendPre(all []middlewareProvider, middlewares []any) []middlewareProvider {
	for _, m := range middlewares {
		if h, ok := m.(PreMiddlewareProvider); ok && h != nil {
			all = append(all, toHandlerProvider(middlewareProvider(h)))
		}
	}
	return all
}

func wrapMiddlewareAppendNormal(all []middlewareProvider, middlewares []any) []middlewareProvider {
	for _, m := range middlewares {
		if _, ok := m.(PreMiddlewareProvider); !ok && !isNilOrFuncNil(m) {
			all = append(all, toHandlerProvider(m))
		}
	}
	return all
}

func wrapMiddlewareAndHandler(useMiddlewares, curMiddlewares, h []any) (_ []middlewareProvider, _ http.HandlerFunc, hasPreMiddlewares bool) {
	if len(h) == 0 {
		panic("no endpoint handler provided")
	}
	if isNilOrFuncNil(h[len(h)-1]) {
		panic("endpoint handler can't be nil")
	}

	handlerProviders := make([]middlewareProvider, 0, len(useMiddlewares)+len(curMiddlewares)+len(h)+1)
	handlerProviders = wrapMiddlewareAppendPre(handlerProviders, useMiddlewares)
	handlerProviders = wrapMiddlewareAppendPre(handlerProviders, curMiddlewares)
	handlerProviders = wrapMiddlewareAppendPre(handlerProviders, h)
	hasPreMiddlewares = len(handlerProviders) > 0
	handlerProviders = wrapMiddlewareAppendNormal(handlerProviders, useMiddlewares)
	handlerProviders = wrapMiddlewareAppendNormal(handlerProviders, curMiddlewares)
	handlerProviders = wrapMiddlewareAppendNormal(handlerProviders, h)

	middlewares := handlerProviders[:len(handlerProviders)-1]
	handlerFunc := handlerProviders[len(handlerProviders)-1](nil).ServeHTTP
	mockPoint := RouterMockPoint(MockAfterMiddlewares)
	if mockPoint != nil {
		middlewares = append(middlewares, mockPoint)
	}
	return middlewares, handlerFunc, hasPreMiddlewares
}

// Methods adds the same handlers for multiple http "methods" (separated by ",").
// If any method is invalid, the lower level router will panic.
func (r *Router) Methods(methods, pattern string, h ...any) {
	middlewares, handlerFunc, _ := wrapMiddlewareAndHandler(r.afterRouting, r.curMiddlewares, h)
	fullPattern := r.getPattern(pattern)
	if strings.Contains(methods, ",") {
		methods := strings.SplitSeq(methods, ",")
		for method := range methods {
			r.chiRouter.With(middlewares...).Method(strings.TrimSpace(method), fullPattern, handlerFunc)
		}
	} else {
		r.chiRouter.With(middlewares...).Method(methods, fullPattern, handlerFunc)
	}
}

// Mount attaches another Router along "/pattern/*"
func (r *Router) Mount(pattern string, subRouter *Router) {
	handlerProviders := make([]middlewareProvider, 0, len(r.afterRouting)+len(r.curMiddlewares))
	handlerProviders = wrapMiddlewareAppendPre(handlerProviders, r.afterRouting)
	handlerProviders = wrapMiddlewareAppendPre(handlerProviders, r.curMiddlewares)
	handlerProviders = wrapMiddlewareAppendNormal(handlerProviders, r.afterRouting)
	handlerProviders = wrapMiddlewareAppendNormal(handlerProviders, r.curMiddlewares)
	r.chiRouter.With(handlerProviders...).Mount(r.getPattern(pattern), subRouter.chiRouter)
}

// Any delegate requests for all methods
func (r *Router) Any(pattern string, h ...any) {
	middlewares, handlerFunc, _ := wrapMiddlewareAndHandler(r.afterRouting, r.curMiddlewares, h)
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
	// TODO: need to move it to the top-level common middleware, otherwise each "Mount" will cause it to be executed multiple times, which is inefficient.
	r.normalizeRequestPath(w, req, r.chiRouter)
}

// NotFound defines a handler to respond whenever a route could not be found.
func (r *Router) NotFound(h http.HandlerFunc) {
	middlewares, handlerFunc, _ := wrapMiddlewareAndHandler(r.afterRouting, r.curMiddlewares, []any{h})
	r.chiRouter.NotFound(func(w http.ResponseWriter, r *http.Request) {
		executeMiddlewaresHandler(w, r, middlewares, handlerFunc)
	})
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

// Combo delegates requests to Combo
func (r *Router) Combo(pattern string, h ...any) *Combo {
	return &Combo{r, pattern, h}
}

// PathGroup creates a group of paths which could be matched by regexp.
// It is only designed to resolve some special cases which chi router can't handle.
// For most cases, it shouldn't be used because it needs to iterate all rules to find the matched one (inefficient).
func (r *Router) PathGroup(pattern string, fn func(g *RouterPathGroup), h ...any) {
	g := &RouterPathGroup{r: r, pathParam: "*"}
	fn(g)
	r.Any(pattern, append(h, g.ServeHTTP)...)
}
