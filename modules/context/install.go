// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"context"
	"net/http"
)

// InstallContext represents a context for installation routes
type InstallContext = DefaultContext

var (
	installContextKey interface{} = "install_context"
)

// WithInstallContext set up install context in request
func WithInstallContext(req *http.Request, ctx *InstallContext) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), installContextKey, ctx))
}

// GetInstallContext retrieves install context from request
func GetInstallContext(req *http.Request) *InstallContext {
	return req.Context().Value(installContextKey).(*InstallContext)
}
