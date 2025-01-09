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
	"code.gitea.io/gitea/modules/web"
	web_types "code.gitea.io/gitea/modules/web/types"
)

// PrivateContext represents a context for private routes
type PrivateContext struct {
	*Base
	Override context.Context

	Repo *Repository
}

func init() {
	web.RegisterResponseStatusProvider[*PrivateContext](func(req *http.Request) web_types.ResponseStatusProvider {
		return req.Context().Value(privateContextKey).(*PrivateContext)
	})
}

// Deadline is part of the interface for context.Context and we pass this to the request context
func (ctx *PrivateContext) Deadline() (deadline time.Time, ok bool) {
	if ctx.Override != nil {
		return ctx.Override.Deadline()
	}
	return ctx.Base.Deadline()
}

// Done is part of the interface for context.Context and we pass this to the request context
func (ctx *PrivateContext) Done() <-chan struct{} {
	if ctx.Override != nil {
		return ctx.Override.Done()
	}
	return ctx.Base.Done()
}

// Err is part of the interface for context.Context and we pass this to the request context
func (ctx *PrivateContext) Err() error {
	if ctx.Override != nil {
		return ctx.Override.Err()
	}
	return ctx.Base.Err()
}

var privateContextKey any = "default_private_context"

// GetPrivateContext returns a context for Private routes
func GetPrivateContext(req *http.Request) *PrivateContext {
	return req.Context().Value(privateContextKey).(*PrivateContext)
}

// PrivateContexter returns apicontext as middleware
func PrivateContexter() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			base := NewBaseContext(w, req)
			ctx := &PrivateContext{Base: base}
			ctx.SetContextValue(privateContextKey, ctx)
			next.ServeHTTP(ctx.Resp, ctx.Req)
		})
	}
}

// OverrideContext overrides the underlying request context for Done() etc.
// This function should be used when there is a need for work to continue even if the request has been cancelled.
// Primarily this affects hook/post-receive and hook/proc-receive both of which need to continue working even if
// the underlying request has timed out from the ssh/http push
func OverrideContext() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// We now need to override the request context as the base for our work because even if the request is cancelled we have to continue this work
			ctx := GetPrivateContext(req)
			var finished func()
			ctx.Override, _, finished = process.GetManager().AddTypedContext(graceful.GetManager().HammerContext(), fmt.Sprintf("PrivateContext: %s", ctx.Req.RequestURI), process.RequestProcessType, true)
			defer finished()
			next.ServeHTTP(ctx.Resp, ctx.Req)
		})
	}
}
