// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routes

import (
	"net/http"

	auth "code.gitea.io/gitea/modules/forms"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers"
)

// InstallRoutes registers the install routes
func InstallRoutes() *web.Route {
	r := BaseRoute()
	r.Use(routers.InstallInit)
	r.Get("/", routers.Install)
	r.Post("/", web.Bind(auth.InstallForm{}), routers.InstallPost)
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, setting.AppURL, 302)
	})
	return r
}
