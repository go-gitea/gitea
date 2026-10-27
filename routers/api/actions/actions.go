// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"net/http"

	"gitea.dev/modules/web"
	"gitea.dev/routers/api/actions/ping"
	"gitea.dev/routers/api/actions/runner"
)

func Routes(prefix string) *web.Router {
	m := web.NewRouter()
	m.Group("/oidc", func() {
		m.Get("/.well-known/openid-configuration", oidcConfiguration)
		m.Get("/jwks", oidcKeys)
		m.Get("/token", oidcToken)
	})

	path, handler := ping.NewPingServiceHandler()
	m.Post(path+"*", http.StripPrefix(prefix, handler).ServeHTTP)

	path, handler = runner.NewRunnerServiceHandler()
	m.Post(path+"*", http.StripPrefix(prefix, handler).ServeHTTP)

	return m
}
