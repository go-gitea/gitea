// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"net/http"

	"gitea.dev/modules/web"
	"gitea.dev/routers/api/codespace/manager"
)

// Routes returns Codespace control-plane API routes.
func Routes(prefix string) *web.Router {
	m := web.NewRouter()

	path, handler := manager.NewManagerServiceHandler()
	m.Post(path+"*", http.StripPrefix(prefix, handler).ServeHTTP)

	return m
}
