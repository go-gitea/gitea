// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	goctx "context"
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/web/routing"
)

type wrappedHandlerFunc func(resp http.ResponseWriter, req *http.Request, others ...wrappedHandlerFunc) (done bool, deferrable func())

func convertHandler(handler interface{}) wrappedHandlerFunc {
	funcInfo := routing.GetFuncInfo(handler)

	switch t := handler.(type) {
	case http.HandlerFunc:
		return func(resp http.ResponseWriter, req *http.Request, others ...wrappedHandlerFunc) (done bool, deferrable func()) {
			traceCtx, cancelSpan := routing.UpdateFuncInfo(req.Context(), funcInfo)
			wrappedReq := req.WithContext(traceCtx)
			if _, ok := resp.(context.ResponseWriter); !ok {
				resp = context.NewResponse(resp)
			}

			// Ensure the span is cancelled even if there is a panic
			func() {
				defer cancelSpan()
				t(resp, wrappedReq)
			}()

			if r, ok := resp.(context.ResponseWriter); ok && r.Status() > 0 {
				done = true
			}
			return done, deferrable
		}
	case func(http.ResponseWriter, *http.Request):
		return func(resp http.ResponseWriter, req *http.Request, others ...wrappedHandlerFunc) (done bool, deferrable func()) {
			traceCtx, cancelSpan := routing.UpdateFuncInfo(req.Context(), funcInfo)
			wrappedReq := req.WithContext(traceCtx)

			// Ensure the span is cancelled even if there is a panic
			func() {
				defer cancelSpan()
				t(resp, wrappedReq)
			}()

			if r, ok := resp.(context.ResponseWriter); ok && r.Status() > 0 {
				done = true
			}
			return done, deferrable
		}

	case func(ctx *context.Context):
		return func(resp http.ResponseWriter, req *http.Request, others ...wrappedHandlerFunc) (done bool, deferrable func()) {
			ctx := context.GetContext(req)
			traceCtx, cancelSpan := routing.UpdateFuncInfo(req.Context(), funcInfo)
			oldReq := ctx.Req
			ctx.Req = req.WithContext(traceCtx)

			// Ensure the span is cancelled even if there is a panic
			func() {
				defer cancelSpan()
				t(ctx)
			}()
			ctx.Req = oldReq

			done = ctx.Written()
			return done, deferrable
		}
	case func(ctx *context.Context) goctx.CancelFunc:
		return func(resp http.ResponseWriter, req *http.Request, others ...wrappedHandlerFunc) (done bool, deferrable func()) {
			ctx := context.GetContext(req)
			traceCtx, cancelSpan := routing.UpdateFuncInfo(req.Context(), funcInfo)
			oldReq := ctx.Req
			ctx.Req = req.WithContext(traceCtx)

			// Ensure the span is cancelled even if there is a panic
			func() {
				defer cancelSpan()
				deferrable = t(ctx)
			}()
			ctx.Req = oldReq

			done = ctx.Written()
			return done, deferrable
		}
	case func(*context.APIContext):
		return func(resp http.ResponseWriter, req *http.Request, others ...wrappedHandlerFunc) (done bool, deferrable func()) {
			ctx := context.GetAPIContext(req)
			traceCtx, cancelSpan := routing.UpdateFuncInfo(req.Context(), funcInfo)
			oldReq := ctx.Req
			ctx.Req = req.WithContext(traceCtx)

			// Ensure the span is cancelled even if there is a panic
			func() {
				defer cancelSpan()
				t(ctx)
			}()
			ctx.Req = oldReq

			done = ctx.Written()
			return done, deferrable
		}
	case func(*context.APIContext) goctx.CancelFunc:
		return func(resp http.ResponseWriter, req *http.Request, others ...wrappedHandlerFunc) (done bool, deferrable func()) {
			ctx := context.GetAPIContext(req)
			traceCtx, cancelSpan := routing.UpdateFuncInfo(req.Context(), funcInfo)
			oldReq := ctx.Req
			ctx.Req = req.WithContext(traceCtx)

			// Ensure the span is cancelled even if there is a panic
			func() {
				defer cancelSpan()
				deferrable = t(ctx)
			}()
			ctx.Req = oldReq

			done = ctx.Written()
			return done, deferrable
		}
	case func(*context.PrivateContext):
		return func(resp http.ResponseWriter, req *http.Request, others ...wrappedHandlerFunc) (done bool, deferrable func()) {
			ctx := context.GetPrivateContext(req)
			traceCtx, cancelSpan := routing.UpdateFuncInfo(req.Context(), funcInfo)
			oldReq := ctx.Req
			ctx.Req = req.WithContext(traceCtx)

			// Ensure the span is cancelled even if there is a panic
			func() {
				defer cancelSpan()
				t(ctx)
			}()
			ctx.Req = oldReq

			done = ctx.Written()
			return done, deferrable
		}
	case func(*context.PrivateContext) goctx.CancelFunc:
		return func(resp http.ResponseWriter, req *http.Request, others ...wrappedHandlerFunc) (done bool, deferrable func()) {
			ctx := context.GetPrivateContext(req)
			traceCtx, cancelSpan := routing.UpdateFuncInfo(req.Context(), funcInfo)
			oldReq := ctx.Req
			ctx.Req = req.WithContext(traceCtx)

			// Ensure the span is cancelled even if there is a panic
			func() {
				defer cancelSpan()
				deferrable = t(ctx)
			}()
			ctx.Req = oldReq

			done = ctx.Written()
			return done, deferrable
		}
	case func(http.Handler) http.Handler:
		return func(resp http.ResponseWriter, req *http.Request, others ...wrappedHandlerFunc) (done bool, deferrable func()) {
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
			if len(others) > 0 {
				next = wrapInternal(others)
			}

			traceCtx, cancelSpan := routing.UpdateFuncInfo(req.Context(), funcInfo)
			wrappedReq := req.WithContext(traceCtx)
			if _, ok := resp.(context.ResponseWriter); !ok {
				resp = context.NewResponse(resp)
			}

			func() {
				defer cancelSpan()
				t(next).ServeHTTP(resp, wrappedReq)
			}()

			if r, ok := resp.(context.ResponseWriter); ok && r.Status() > 0 {
				done = true
			}
			return done, deferrable
		}
	case *NamedHandlerFunc:
		return func(resp http.ResponseWriter, req *http.Request, others ...wrappedHandlerFunc) (done bool, deferrable func()) {
			traceCtx, cancelSpan := routing.UpdateFuncInfo(req.Context(), t.funcInfo)
			wrappedReq := req.WithContext(traceCtx)
			if _, ok := resp.(context.ResponseWriter); !ok {
				resp = context.NewResponse(resp)
			}

			// Ensure the span is cancelled even if there is a panic
			func() {
				defer cancelSpan()
				t.HandlerFunc(resp, wrappedReq)
			}()

			if r, ok := resp.(context.ResponseWriter); ok && r.Status() > 0 {
				done = true
			}
			return done, deferrable
		}
	default:
		panic(fmt.Sprintf("Unsupported handler type: %#v", t))
	}
}

// NamedHandlerFunc represents a handler func that has a friendly name
type NamedHandlerFunc struct {
	http.HandlerFunc
	friendlyName string
	funcInfo     *routing.FuncInfo
}

// WithFriendlyName gives a handler func a FriendlyName
func WithFriendlyName(handler http.HandlerFunc, friendlyName string) *NamedHandlerFunc {
	return &NamedHandlerFunc{
		HandlerFunc:  handler,
		friendlyName: friendlyName,
		funcInfo:     routing.GetFuncInfo(handler, friendlyName),
	}
}
