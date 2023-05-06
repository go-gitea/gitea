// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/web/middleware"
)

// PrivateContext represents a context for private routes
type PrivateContext struct {
	*Context
	Override context.Context
}

// Deadline is part of the interface for context.Context and we pass this to the request context
func (ctx *PrivateContext) Deadline() (deadline time.Time, ok bool) {
	if ctx.Override != nil {
		return ctx.Override.Deadline()
	}
	return ctx.Req.Context().Deadline()
}

// Done is part of the interface for context.Context and we pass this to the request context
func (ctx *PrivateContext) Done() <-chan struct{} {
	if ctx.Override != nil {
		return ctx.Override.Done()
	}
	return ctx.Req.Context().Done()
}

// Err is part of the interface for context.Context and we pass this to the request context
func (ctx *PrivateContext) Err() error {
	if ctx.Override != nil {
		return ctx.Override.Err()
	}
	return ctx.Req.Context().Err()
}

var privateContextKey interface{} = "default_private_context"

// WithPrivateContext set up private context in request
func WithPrivateContext(req *http.Request, ctx *PrivateContext) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), privateContextKey, ctx))
}

// GetPrivateContext returns a context for Private routes
func GetPrivateContext(req *http.Request) *PrivateContext {
	return req.Context().Value(privateContextKey).(*PrivateContext)
}

// PrivateContexter returns apicontext as middleware
func PrivateContexter() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := &PrivateContext{
				Context: &Context{
					Resp: NewResponse(w),
					Data: middleware.GetContextData(req.Context()),
				},
			}
			defer ctx.Close()

			ctx.Req = WithPrivateContext(req, ctx)
			ctx.Data["Context"] = ctx
			next.ServeHTTP(ctx.Resp, ctx.Req)
		})
	}
}

// OverrideContext overrides the underlying request context for Done() etc.
// This function should be used when there is a need for work to continue even if the request has been cancelled.
// Primarily this affects hook/post-receive and hook/proc-receive both of which need to continue working even if
// the underlying request has timed out from the ssh/http push
func OverrideContext(ctx *PrivateContext) (cancel context.CancelFunc) {
	// We now need to override the request context as the base for our work because even if the request is cancelled we have to continue this work
	ctx.Override, _, cancel = process.GetManager().AddTypedContext(graceful.GetManager().HammerContext(), fmt.Sprintf("PrivateContext: %s", ctx.Req.RequestURI), process.RequestProcessType, true)
	return cancel
}
