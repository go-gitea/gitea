// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	goctx "context"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/web/routing"
)

// Wrap converts all kinds of routes to standard library one
func Wrap(handlers ...interface{}) http.HandlerFunc {
	if len(handlers) == 0 {
		panic("No handlers found")
	}

	ourHandlers := make([]wrappedHandlerFunc, 0, len(handlers))

	for _, handler := range handlers {
		ourHandlers = append(ourHandlers, convertHandler(handler))
	}
	return wrapInternal(ourHandlers)
}

func wrapInternal(handlers []wrappedHandlerFunc) http.HandlerFunc {
	return func(resp http.ResponseWriter, req *http.Request) {
		var defers []func()
		defer func() {
			for i := len(defers) - 1; i >= 0; i-- {
				defers[i]()
			}
		}()
		for i := 0; i < len(handlers); i++ {
			handler := handlers[i]
			others := handlers[i+1:]
			done, deferrable := handler(resp, req, others...)
			if deferrable != nil {
				defers = append(defers, deferrable)
			}
			if done {
				return
			}
		}
	}
}

// Middle wrap a context function as a chi middleware
func Middle(f func(ctx *context.Context)) func(next http.Handler) http.Handler {
	funcInfo := routing.GetFuncInfo(f)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			ctx := context.GetContext(req)
			traceCtx, cancelSpan := routing.UpdateFuncInfo(req.Context(), funcInfo)
			ctx.Req = req.WithContext(traceCtx)

			// Ensure the span is cancelled even if there is a panic
			func() {
				defer cancelSpan()
				f(ctx)
			}()

			if ctx.Written() {
				return
			}
			next.ServeHTTP(ctx.Resp, req)
		})
	}
}

// MiddleCancel wrap a context function as a chi middleware
func MiddleCancel(f func(ctx *context.Context) goctx.CancelFunc) func(netx http.Handler) http.Handler {
	funcInfo := routing.GetFuncInfo(f)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			ctx := context.GetContext(req)
			traceCtx, cancelSpan := routing.UpdateFuncInfo(req.Context(), funcInfo)
			ctx.Req = req.WithContext(traceCtx)

			// Ensure the span is cancelled even if there is a panic
			var deferrable goctx.CancelFunc
			func() {
				defer cancelSpan()
				deferrable = f(ctx)
			}()

			if deferrable != nil {
				defer deferrable()
			}
			if ctx.Written() {
				return
			}
			next.ServeHTTP(ctx.Resp, req)
		})
	}
}

// MiddleAPI wrap a context function as a chi middleware
func MiddleAPI(f func(ctx *context.APIContext)) func(next http.Handler) http.Handler {
	funcInfo := routing.GetFuncInfo(f)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			ctx := context.GetAPIContext(req)
			traceCtx, cancelSpan := routing.UpdateFuncInfo(req.Context(), funcInfo)
			ctx.Req = req.WithContext(traceCtx)

			// Ensure the span is cancelled even if there is a panic
			func() {
				defer cancelSpan()
				f(ctx)
			}()

			if ctx.Written() {
				return
			}

			next.ServeHTTP(ctx.Resp, req)
		})
	}
}

// MiddleHandler wraps a provided handler function with or without a prefix
func MiddleHandler(handler http.HandlerFunc, friendlyName ...string) func(next http.Handler) http.Handler {
	funcInfo := routing.GetFuncInfo(handler, friendlyName...)

	return func(_ http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			traceCtx, cancelSpan := routing.UpdateFuncInfo(req.Context(), funcInfo)
			wrappedReq := req.WithContext(traceCtx)

			defer cancelSpan()
			handler(resp, wrappedReq)
		})
	}
}

// WrapWithPrefix wraps a provided handler function at a prefix
func WrapWithPrefix(pathPrefix string, handler http.HandlerFunc, friendlyName ...string) func(next http.Handler) http.Handler {
	funcInfo := routing.GetFuncInfo(handler, friendlyName...)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if !strings.HasPrefix(req.URL.Path, pathPrefix) {
				next.ServeHTTP(resp, req)
				return
			}
			traceCtx, cancelSpan := routing.UpdateFuncInfo(req.Context(), funcInfo)
			wrappedReq := req.WithContext(traceCtx)

			defer cancelSpan()
			handler(resp, wrappedReq)
		})
	}
}
