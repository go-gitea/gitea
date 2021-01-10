// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routes

import (
	"code.gitea.io/gitea/modules/context"
	auth "code.gitea.io/gitea/modules/forms"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers"

	"gitea.com/go-chi/binding"
)

// InstallRoutes registers the install routes
func InstallRoutes() *Route {
	r := BaseRoute()
	r.Combo("/", routers.InstallInit).Get(routers.Install).
		Post(binding.BindIgnErr(auth.InstallForm{}), routers.InstallPost)
	r.NotFound(func(ctx *context.Context) {
		ctx.Redirect(setting.AppURL, 302)
	})
	return r
}
