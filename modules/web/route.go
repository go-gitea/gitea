// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"net/http"
	"strings"

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
func SetForm(dataStore middleware.ContextDataStore, obj any) {
	dataStore.GetData()["__form"] = obj
}

// GetForm returns the validate form information
func GetForm(dataStore middleware.ContextDataStore) any {
	return dataStore.GetData()["__form"]
}

// Route defines a route based on chi's router
type Route struct {
	R              chi.Router
	curGroupPrefix string
	curMiddlewares []any
}

// NewRoute creates a new route
func NewRoute() *Route {
	r := chi.NewRouter()
	return &Route{R: r}
}

// Use supports two middlewares
func (r *Route) Use(middlewares ...any) {
	for _, m := range middlewares {
		if m != nil {
			r.R.Use(toHandlerProvider(m))
		}
	}
}

// Group mounts a sub-Router along a `pattern` string.
func (r *Route) Group(pattern string, fn func(), middlewares ...any) {
	previousGroupPrefix := r.curGroupPrefix
	previousMiddlewares := r.curMiddlewares
	r.curGroupPrefix += pattern
	r.curMiddlewares = append(r.curMiddlewares, middlewares...)

	fn()

	r.curGroupPrefix = previousGroupPrefix
	r.curMiddlewares = previousMiddlewares
}

func (r *Route) getPattern(pattern string) string {
	newPattern := r.curGroupPrefix + pattern
	if !strings.HasPrefix(newPattern, "/") {
		newPattern = "/" + newPattern
	}
	if newPattern == "/" {
		return newPattern
	}
	return strings.TrimSuffix(newPattern, "/")
}

func (r *Route) wrapMiddlewareAndHandler(h []any) ([]func(http.Handler) http.Handler, http.HandlerFunc) {
	handlerProviders := make([]func(http.Handler) http.Handler, 0, len(r.curMiddlewares)+len(h)+1)
	for _, m := range r.curMiddlewares {
		if m != nil {
			handlerProviders = append(handlerProviders, toHandlerProvider(m))
		}
	}
	for _, m := range h {
		if h != nil {
			handlerProviders = append(handlerProviders, toHandlerProvider(m))
		}
	}
	middlewares := handlerProviders[:len(handlerProviders)-1]
	handlerFunc := handlerProviders[len(handlerProviders)-1](nil).ServeHTTP
	mockPoint := RouteMockPoint(MockAfterMiddlewares)
	if mockPoint != nil {
		middlewares = append(middlewares, mockPoint)
	}
	return middlewares, handlerFunc
}

func (r *Route) Methods(method, pattern string, h ...any) {
	middlewares, handlerFunc := r.wrapMiddlewareAndHandler(h)
	fullPattern := r.getPattern(pattern)
	if strings.Contains(method, ",") {
		methods := strings.Split(method, ",")
		for _, method := range methods {
			r.R.With(middlewares...).Method(strings.TrimSpace(method), fullPattern, handlerFunc)
		}
	} else {
		r.R.With(middlewares...).Method(method, fullPattern, handlerFunc)
	}
}

// Mount attaches another Route along ./pattern/*
func (r *Route) Mount(pattern string, subR *Route) {
	subR.Use(r.curMiddlewares...)
	r.R.Mount(r.getPattern(pattern), subR.R)
}

// Any delegate requests for all methods
func (r *Route) Any(pattern string, h ...any) {
	middlewares, handlerFunc := r.wrapMiddlewareAndHandler(h)
	r.R.With(middlewares...).HandleFunc(r.getPattern(pattern), handlerFunc)
}

// Delete delegate delete method
func (r *Route) Delete(pattern string, h ...any) {
	r.Methods("DELETE", pattern, h...)
}

// Get delegate get method
func (r *Route) Get(pattern string, h ...any) {
	r.Methods("GET", pattern, h...)
}

// GetOptions delegate get and options method
func (r *Route) GetOptions(pattern string, h ...any) {
	r.Methods("GET,OPTIONS", pattern, h...)
}

// PostOptions delegate post and options method
func (r *Route) PostOptions(pattern string, h ...any) {
	r.Methods("POST,OPTIONS", pattern, h...)
}

// Head delegate head method
func (r *Route) Head(pattern string, h ...any) {
	r.Methods("HEAD", pattern, h...)
}

// Post delegate post method
func (r *Route) Post(pattern string, h ...any) {
	r.Methods("POST", pattern, h...)
}

// Put delegate put method
func (r *Route) Put(pattern string, h ...any) {
	r.Methods("PUT", pattern, h...)
}

// Patch delegate patch method
func (r *Route) Patch(pattern string, h ...any) {
	r.Methods("PATCH", pattern, h...)
}

// ServeHTTP implements http.Handler
func (r *Route) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.R.ServeHTTP(w, req)
}

// NotFound defines a handler to respond whenever a route could not be found.
func (r *Route) NotFound(h http.HandlerFunc) {
	r.R.NotFound(h)
}

// Combo delegates requests to Combo
func (r *Route) Combo(pattern string, h ...any) *Combo {
	return &Combo{r, pattern, h}
}

// Combo represents a tiny group routes with same pattern
type Combo struct {
	r       *Route
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
