// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routes

import (
	"net/http"
	"reflect"

	"code.gitea.io/gitea/modules/context"

	"gitea.com/go-chi/binding"
	"github.com/go-chi/chi"
)

// Wrap converts an install route to a chi route
func Wrap(handlers ...interface{}) http.HandlerFunc {
	if len(handlers) == 0 {
		panic("No handlers found")
	}
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		ctx := context.GetContext(req)
		ctx.Resp = resp
		for _, handler := range handlers {
			switch t := handler.(type) {
			case func(ctx *context.Context):
				// TODO: if ctx.Written return immediately
				hanlder(ctx)
			case func(resp http.ResponseWriter, req *http.Request):
				t(resp, req)
			}
		}
	})
}

// Middle wrap a function to middle
func Middle(f func(ctx *context.Context)) func(netx http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			Wrap(f)(resp, req)
			next.ServeHTTP(resp, req)
		})
	}
}

// Bind binding an obj to a handler
func Bind(obj interface{}, handler func(ctx *context.Context, form interface{})) http.HandlerFunc {
	var tp = reflect.TypeOf(obj).Elem()
	return Wrap(func(ctx *context.Context) {
		var theObj = reflect.New(tp).Interface() // create a new form obj for every request but not use obj directly
		binding.Bind(ctx.Req, theObj)
		handler(ctx, theObj)
	})
}

// Group groups router and middles
func Group(f func(r chi.Router), middles ...func(netx http.Handler) http.Handler) func(r chi.Router) {
	return func(r chi.Router) {
		for _, middle := range middles {
			r.Use(middle)
		}
		f(r)
	}
}

type CtxFunc func(ctx *context.Context)

type Route struct {
	R chi.Router
}

func NewRoute() *Route {
	r := chi.NewRouter()
	return &Route{
		R: r,
	}
}

func (r *Route) Use(middlewares ...interface{}) {
	for _, middle := range middlewares {
		switch t := middle.(type) {
		case func(http.Handler) http.Handler:
			r.R.Use(t)
		case func(*context.Context):
			r.R.Use(Middle(t))
		}
	}
}

// Route mounts a sub-Router along a `pattern`` string.
func (r *Route) Group(pattern string, fn func(r *Route)) *Route {
	r.Route(pattern, Group())
}

// Mount attaches another http.Handler along ./pattern/*
func (r *Route) Mount(pattern string, subR *Route) {
	r.R.Mount(pattern, subR.R)
}

func (r *Route) Delete(pattern string, h ...CtxFunc) {
	r.R.Delete(pattern, Wrap(h...))
}

func (r *Route) Get(pattern string, h ...CtxFunc) {
	r.R.Get(pattern, Wrap(h...))
}

func (r *Route) Head(pattern string, h ...CtxFunc) {
	r.R.Head(pattern, Wrap(h...))
}

func (r *Route) Post(pattern string, h ...CtxFunc) {
	r.R.Post(pattern, Wrap(h...))
}

func (r *Route) Put(pattern string, h ...CtxFunc) {
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
