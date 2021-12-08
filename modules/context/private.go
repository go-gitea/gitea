// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"context"
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
)

// PrivateContext represents a context for private routes
type PrivateContext struct {
	*BaseContext
	Repository *models.Repository
	GitRepo    *git.Repository
}

var (
	privateContextKey interface{} = "default_private_context"
)

// NewPrivateContext creates a new private context
func NewPrivateContext(resp http.ResponseWriter, req *http.Request, data map[string]interface{}) *PrivateContext {
	return &PrivateContext{
		BaseContext: NewBaseContext(resp, req, map[string]interface{}{}),
	}
}

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
			ctx := NewPrivateContext(w, req, map[string]interface{}{})
			ctx.Req = WithPrivateContext(req, ctx)
			next.ServeHTTP(ctx.Resp, ctx.Req)
		})
	}
}
