// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package app

import (
	"errors"
	"net/http"

	"code.gitea.io/gitea/models/application"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/renderhelper"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
)

const (
	tplView              templates.TplName = "apps/view"
	tplViewInstallations templates.TplName = "apps/view_installations"
	tplViewInstallation  templates.TplName = "apps/view_installation_target"
)

func ViewApp(ctx *context.Context) {
	ctx.Data["Title"] = ctx.GiteaApp.App.Name

	appExt := ctx.GiteaApp.App.AppExternalData()

	rctx := renderhelper.NewRenderContextApplication(ctx)

	var err error
	appExt.RenderedReadme, err = markdown.RenderString(rctx, appExt.Readme)
	if err != nil {
		ctx.ServerError("markdown.RenderString", err)
		return
	}

	ctx.HTML(http.StatusOK, tplView)
}

func checkAndLoadInstallationByTargetID(ctx *context.Context, targetID int64) *application.AppInstallation {
	var owner *user_model.User

	switch targetID {
	case user_model.SystemAdminUserID:
		if !ctx.Doer.IsAdmin {
			ctx.NotFound(errors.New("no permission to view system admin installations"))
			return nil
		}
		owner = user_model.NewSystemAdminUser()
		ctx.Data["IsSystemAdminInstallation"] = true
	case ctx.Doer.ID:
		owner = ctx.Doer
		ctx.Data["IsUserInstallation"] = true
	default:
		org, err := organization.GetOrgByID(ctx, targetID)
		if err != nil {
			ctx.NotFound(err)
			return nil
		}

		if isAdmin, err := org.IsOrgAdmin(ctx, ctx.Doer.ID); err != nil {
			ctx.ServerError("IsOrgAdmin", err)
			return nil
		} else if !isAdmin {
			ctx.NotFound(errors.New("no permission to view this organization's installations"))
			return nil
		}
		owner = org.AsUser()
		ctx.Data["IsOrgInstallation"] = true
	}

	installation, err := ctx.GiteaApp.App.GetInstallationByOwnerID(ctx, targetID)
	if err != nil {
		ctx.ServerError("GetInstallationByOwnerID", err)
		return nil
	}
	installation.Owner = owner

	return installation
}

func ViewAppInstallationOnTarget(ctx *context.Context, targetID int64) {
	install := checkAndLoadInstallationByTargetID(ctx, targetID)
	if install == nil {
		return
	}

	ctx.Data["Title"] = ctx.Tr("apps.installations.on_target.title", ctx.GiteaApp.App.Name, install.Owner.DisplayName())
	ctx.Data["Installation"] = install

	ctx.HTML(http.StatusOK, tplViewInstallation)
}

func ViewAppInstallations(ctx *context.Context) {
	if !ctx.IsSigned {
		ctx.NotFound(errors.New("request sigin"))
		return
	}

	if id := ctx.FormInt64("target_id"); id != 0 {
		ViewAppInstallationOnTarget(ctx, id)
		return
	}

	ctx.Data["Title"] = ctx.Tr("apps.installations.title", ctx.GiteaApp.App.Name)

	actorIDs := make([]int64, 0, 5)
	actorIDs = append(actorIDs, ctx.Doer.ID)

	orgs, err := organization.GetOrgsOwnedByUserID(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetOrgsOwnedByUserID", err)
		return
	}
	for _, org := range orgs {
		actorIDs = append(actorIDs, org.ID)
	}

	if ctx.Doer.IsAdmin {
		actorIDs = append(actorIDs, user_model.SystemAdminUserID)
	}

	installations, err := ctx.GiteaApp.App.GetInstallationByOwnerIDs(ctx, actorIDs)
	if err != nil {
		ctx.ServerError("GetInstallationByOwnerIDs", err)
		return
	}

	installations[ctx.Doer.ID].Owner = ctx.Doer
	for _, org := range orgs {
		installations[org.ID].Owner = org.AsUser()
	}

	installationsList := make([]*application.AppInstallation, 0, len(installations))
	for _, v := range installations {
		installationsList = append(installationsList, v)
	}
	if ctx.Doer.IsAdmin {
		systemAdmin := user_model.NewSystemAdminUser()
		installations[systemAdmin.ID].Owner = systemAdmin
	}

	ctx.Data["Installations"] = installationsList

	ctx.HTML(http.StatusOK, tplViewInstallations)
}
