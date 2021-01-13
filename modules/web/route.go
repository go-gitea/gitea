// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/middlewares"

	"gitea.com/go-chi/binding"
	"github.com/go-chi/chi"
)

// Wrap converts routes to stand one
func Wrap(handlers ...interface{}) http.HandlerFunc {
	if len(handlers) == 0 {
		panic("No handlers found")
	}
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		for _, handler := range handlers {
			switch t := handler.(type) {
			case http.HandlerFunc:
				t(resp, req)
			case func(ctx *context.Context):
				ctx := context.GetContext(req)
				t(ctx)
				if ctx.Written() {
					return
				}
			case func(*context.APIContext):
				ctx := context.GetAPIContext(req)
				t(ctx)
				if ctx.Written() {
					return
				}
			default:
				panic(fmt.Sprintf("No supported handler type: %#v", t))
			}
		}
	})
}

// Middle wrap a function to middle
func Middle(f func(ctx *context.Context)) func(netx http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			Wrap(f)(resp, req)
			ctx := context.GetContext(req)
			if ctx.Written() {
				return
			}
			next.ServeHTTP(resp, req)
		})
	}
}

// Bind binding an obj to a handler
func Bind(obj interface{}) http.HandlerFunc {
	var tp = reflect.TypeOf(obj)
	if tp.Kind() == reflect.Ptr {
		tp = tp.Elem()
	}
	if tp.Kind() != reflect.Struct {
		panic("Only structs are allowed to bind")
	}
	return Wrap(func(ctx *context.Context) {
		var theObj = reflect.New(tp).Interface() // create a new form obj for every request but not use obj directly
		binding.Bind(ctx.Req, theObj)
		SetForm(ctx, theObj)
		middlewares.AssignForm(theObj, ctx.Data)
	})
}

// SetForm set the form object
func SetForm(data middlewares.DataStore, obj interface{}) {
	data.GetData()["__form"] = obj
}

// GetForm returns the validate form information
func GetForm(data middlewares.DataStore) interface{} {
	return data.GetData()["__form"]
}

// Route defines a route based on chi's router
type Route struct {
	R chi.Router
}

// NewRoute creates a new route
func NewRoute() *Route {
	r := chi.NewRouter()
	return &Route{
		R: r,
	}
}

// Use supports two middlewares
func (r *Route) Use(middlewares ...interface{}) {
	for _, middle := range middlewares {
		switch t := middle.(type) {
		case func(http.Handler) http.Handler:
			r.R.Use(t)
		case func(*context.Context):
			r.R.Use(Middle(t))
		default:
			panic(fmt.Sprintf("Unsupported middleware type: %#v", t))
		}
	}
}

// Group mounts a sub-Router along a `pattern`` string.
func (r *Route) Group(pattern string, fn func(r *Route), middlewares ...interface{}) {
	if pattern == "" {
		pattern = "/"
	}
	r.R.Route(pattern, func(r chi.Router) {
		sr := &Route{
			R: r,
		}
		sr.Use(middlewares...)
		fn(sr)
	})
}

// Mount attaches another http.Handler along ./pattern/*
func (r *Route) Mount(pattern string, subR *Route) {
	if pattern == "" {
		pattern = "/"
	}
	r.R.Mount(pattern, subR.R)
}

// Any delegate all methods
func (r *Route) Any(pattern string, h ...interface{}) {
	if pattern == "" {
		pattern = "/"
	}
	r.R.HandleFunc(pattern, Wrap(h...))
}

// Route delegate special methods
func (r *Route) Route(pattern string, methods string, h ...interface{}) {
	if pattern == "" {
		pattern = "/"
	}
	ms := strings.Split(methods, ",")
	for _, method := range ms {
		r.R.MethodFunc(strings.TrimSpace(method), pattern, Wrap(h...))
	}
}

// Delete delegate delete method
func (r *Route) Delete(pattern string, h ...interface{}) {
	if pattern == "" {
		pattern = "/"
	}
	r.R.Delete(pattern, Wrap(h...))
}

// Get delegate get method
func (r *Route) Get(pattern string, h ...interface{}) {
	if pattern == "" {
		pattern = "/"
	}
	r.R.Get(pattern, Wrap(h...))
}

// Head delegate head method
func (r *Route) Head(pattern string, h ...interface{}) {
	if pattern == "" {
		pattern = "/"
	}
	r.R.Head(pattern, Wrap(h...))
}

// Post delegate post method
func (r *Route) Post(pattern string, h ...interface{}) {
	if pattern == "" {
		pattern = "/"
	}
	r.R.Post(pattern, Wrap(h...))
}

// Put delegate put method
func (r *Route) Put(pattern string, h ...interface{}) {
	if pattern == "" {
		pattern = "/"
	}
	r.R.Put(pattern, Wrap(h...))
}

// Patch delegate patch method
func (r *Route) Patch(pattern string, h ...interface{}) {
	if pattern == "" {
		pattern = "/"
	}
	r.R.Patch(pattern, Wrap(h...))
}

// ServeHTTP implements http.Handler
func (r *Route) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.R.ServeHTTP(w, req)
}

// NotFound defines a handler to respond whenever a route could
// not be found.
func (r *Route) NotFound(h http.HandlerFunc) {
	r.R.NotFound(h)
}

// MethodNotAllowed defines a handler to respond whenever a method is
// not allowed.
func (r *Route) MethodNotAllowed(h http.HandlerFunc) {
	r.R.MethodNotAllowed(h)
}

// Combo deletegate requests to Combo
func (r *Route) Combo(pattern string, h ...interface{}) *Combo {
	if pattern == "" {
		pattern = "/"
	}
	return &Combo{r, pattern, h}
}

// Combo represents a tiny group routes with same pattern
type Combo struct {
	r       *Route
	pattern string
	h       []interface{}
}

// Get deletegate Get method
func (c *Combo) Get(h ...interface{}) *Combo {
	c.r.Get(c.pattern, append(c.h, h...))
	return c
}

// Post deletegate Post method
func (c *Combo) Post(h ...interface{}) *Combo {
	c.r.Post(c.pattern, append(c.h, h...))
	return c
}

// Delete deletegate Delete method
func (c *Combo) Delete(h ...interface{}) *Combo {
	c.r.Delete(c.pattern, append(c.h, h...))
	return c
}

// Put deletegate Put method
func (c *Combo) Put(h ...interface{}) *Combo {
	c.r.Put(c.pattern, append(c.h, h...))
	return c
}

// Patch deletegate Patch method
func (c *Combo) Patch(h ...interface{}) *Combo {
	c.r.Patch(c.pattern, append(c.h, h...))
	return c
}
