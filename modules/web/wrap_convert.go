// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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
			routing.UpdateFuncInfo(req.Context(), funcInfo)
			t(resp, req)
			if r, ok := resp.(context.ResponseWriter); ok && r.Status() > 0 {
				done = true
			}
			return
		}
	case func(http.ResponseWriter, *http.Request):
		return func(resp http.ResponseWriter, req *http.Request, others ...wrappedHandlerFunc) (done bool, deferrable func()) {
			routing.UpdateFuncInfo(req.Context(), funcInfo)
			t(resp, req)
			if r, ok := resp.(context.ResponseWriter); ok && r.Status() > 0 {
				done = true
			}
			return
		}

	case func(ctx *context.Context):
		return func(resp http.ResponseWriter, req *http.Request, others ...wrappedHandlerFunc) (done bool, deferrable func()) {
			routing.UpdateFuncInfo(req.Context(), funcInfo)
			ctx := context.GetContext(req)
			t(ctx)
			done = ctx.Written()
			return
		}
	case func(ctx *context.Context) goctx.CancelFunc:
		return func(resp http.ResponseWriter, req *http.Request, others ...wrappedHandlerFunc) (done bool, deferrable func()) {
			routing.UpdateFuncInfo(req.Context(), funcInfo)
			ctx := context.GetContext(req)
			deferrable = t(ctx)
			done = ctx.Written()
			return
		}
	case func(*context.APIContext):
		return func(resp http.ResponseWriter, req *http.Request, others ...wrappedHandlerFunc) (done bool, deferrable func()) {
			routing.UpdateFuncInfo(req.Context(), funcInfo)
			ctx := context.GetAPIContext(req)
			t(ctx)
			done = ctx.Written()
			return
		}
	case func(*context.APIContext) goctx.CancelFunc:
		return func(resp http.ResponseWriter, req *http.Request, others ...wrappedHandlerFunc) (done bool, deferrable func()) {
			routing.UpdateFuncInfo(req.Context(), funcInfo)
			ctx := context.GetAPIContext(req)
			deferrable = t(ctx)
			done = ctx.Written()
			return
		}
	case func(*context.PrivateContext):
		return func(resp http.ResponseWriter, req *http.Request, others ...wrappedHandlerFunc) (done bool, deferrable func()) {
			routing.UpdateFuncInfo(req.Context(), funcInfo)
			ctx := context.GetPrivateContext(req)
			t(ctx)
			done = ctx.Written()
			return
		}
	case func(*context.PrivateContext) goctx.CancelFunc:
		return func(resp http.ResponseWriter, req *http.Request, others ...wrappedHandlerFunc) (done bool, deferrable func()) {
			routing.UpdateFuncInfo(req.Context(), funcInfo)
			ctx := context.GetPrivateContext(req)
			deferrable = t(ctx)
			done = ctx.Written()
			return
		}
	case func(http.Handler) http.Handler:
		return func(resp http.ResponseWriter, req *http.Request, others ...wrappedHandlerFunc) (done bool, deferrable func()) {
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
			if len(others) > 0 {
				next = wrapInternal(others)
			}
			routing.UpdateFuncInfo(req.Context(), funcInfo)
			t(next).ServeHTTP(resp, req)
			return
		}
	default:
		panic(fmt.Sprintf("Unsupported handler type: %#v", t))
	}
}
