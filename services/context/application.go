// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"errors"

	application_model "code.gitea.io/gitea/models/application"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

type Application struct {
	App *application_model.Application

	IsAdmin    bool
	CanInstall bool
}

type ApplicationAssignmentOptions struct {
	RequireAdmin      bool
	RequireCanInstall bool
}

func ApplicationAssignment(opts ApplicationAssignmentOptions) func(ctx *Context) {
	return func(ctx *Context) {
		if ctx.Data["GiteaApp"] != nil {
			setting.PanicInDevOrTesting("Application should not be executed twice")
		}

		appname := ctx.PathParam("appname")

		app, err := application_model.GetAppByName(ctx, appname)
		if err != nil {
			if errors.Is(err, util.ErrNotExist) {
				ctx.NotFound(err)
			} else {
				ctx.ServerError("GetAppByName", err)
			}
			return
		}

		if ok, err := app.CanViewBy(ctx, ctx.Doer); err != nil {
			ctx.ServerError("CanViewBy", err)
			return
		} else if !ok {
			ctx.NotFound(err)
			return
		}

		ctx.GiteaApp = &Application{App: app}
		ctx.Data["GiteaApp"] = app
		ctx.ContextUser = app.AsUser()

		ctx.GiteaApp.IsAdmin = app.IsAppManager(ctx.Doer)

		if opts.RequireAdmin && !ctx.GiteaApp.IsAdmin {
			ctx.NotFound(nil)
			return
		}

		ctx.Data["IsAppManager"] = ctx.GiteaApp.IsAdmin
	}

	// ctx.GiteaApp.CanInstall = ctx.GiteaApp.CanInstall(ctx.Doer)
}
