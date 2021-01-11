// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"fmt"
	"net/http"
	"reflect"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/middlewares"

	"gitea.com/go-chi/binding"
	"github.com/go-chi/chi"
)

// CtxFunc defines default context function
type CtxFunc func(ctx *context.Context)

// Wrap converts routes to stand one
func Wrap(handlers ...interface{}) http.HandlerFunc {
	if len(handlers) == 0 {
		panic("No handlers found")
	}
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		for _, handler := range handlers {
			switch t := handler.(type) {
			case CtxFunc:
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
	var tp = reflect.TypeOf(obj).Elem()
	return Wrap(func(ctx *context.Context) {
		var theObj = reflect.New(tp).Interface() // create a new form obj for every request but not use obj directly
		binding.Bind(ctx.Req, theObj)
		SetForm(ctx, theObj)
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
	sr := NewRoute()
	sr.Use(middlewares...)
	fn(sr)
	r.Mount(pattern, sr)
}

// Mount attaches another http.Handler along ./pattern/*
func (r *Route) Mount(pattern string, subR *Route) {
	r.R.Mount(pattern, subR.R)
}

// Delete delegate delete method
func (r *Route) Delete(pattern string, h ...interface{}) {
	r.R.Delete(pattern, Wrap(h...))
}

// Get delegate get method
func (r *Route) Get(pattern string, h ...interface{}) {
	r.R.Get(pattern, Wrap(h...))
}

// Head delegate head method
func (r *Route) Head(pattern string, h ...interface{}) {
	r.R.Head(pattern, Wrap(h...))
}

// Post delegate post method
func (r *Route) Post(pattern string, h ...interface{}) {
	r.R.Post(pattern, Wrap(h...))
}

// Put delegate put method
func (r *Route) Put(pattern string, h ...interface{}) {
	r.R.Put(pattern, Wrap(h...))
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
