// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"
	"net/url"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	actions_shared "code.gitea.io/gitea/routers/web/shared/actions"
)

const (
	// tplSettingsActions template path for render actions settings
	tplSettingsActions base.TplName = "org/settings/actions"
	// tplSettingsRunnersEdit template path for render runners edit settings
	tplSettingsRunnersEdit base.TplName = "org/settings/runners_edit"
)

// Runners render settings/actions/runners page for organization level
func Runners(ctx *context.Context) {
	ctx.Data["PageIsOrgSettingsRunners"] = true

	ctx.Data["Title"] = ctx.Tr("actions.actions")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageType"] = "runners"
	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	opts := actions_model.FindRunnerOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: 100,
		},
		Sort:          ctx.Req.URL.Query().Get("sort"),
		Filter:        ctx.Req.URL.Query().Get("q"),
		OwnerID:       ctx.Org.Organization.ID,
		WithAvailable: true,
	}

	actions_shared.RunnersList(ctx, opts)
	ctx.HTML(http.StatusOK, tplSettingsActions)
}

// ResetRunnerRegistrationToken reset runner registration token
func ResetRunnerRegistrationToken(ctx *context.Context) {
	actions_shared.RunnerResetRegistrationToken(ctx,
		ctx.Org.Organization.ID, 0,
		ctx.Org.OrgLink+"/settings/actions/runners")
}

// RunnersEdit render runner edit page for organization level
func RunnersEdit(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("org.runners.edit")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsOrgSettingsRunners"] = true
	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	actions_shared.RunnerDetails(ctx, page,
		ctx.ParamsInt64(":runnerid"), ctx.Org.Organization.ID, 0,
	)

	ctx.HTML(http.StatusOK, tplSettingsRunnersEdit)
}

// RunnersEditPost response for editing runner
func RunnersEditPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("org.runners.edit")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsOrgSettingsRunners"] = true
	actions_shared.RunnerDetailsEditPost(ctx, ctx.ParamsInt64(":runnerid"),
		ctx.Org.Organization.ID, 0,
		ctx.Org.OrgLink+"/settings/actions/runners/"+url.PathEscape(ctx.Params(":runnerid")))
}

// RunnerDeletePost response for deleting runner
func RunnerDeletePost(ctx *context.Context) {
	actions_shared.RunnerDeletePost(ctx,
		ctx.ParamsInt64(":runnerid"),
		ctx.Org.OrgLink+"/settings/actions/runners",
		ctx.Org.OrgLink+"/settings/actions/runners/"+url.PathEscape(ctx.Params(":runnerid")))
}

func RedirectToDefaultSetting(ctx *context.Context) {
	ctx.Redirect(ctx.Org.OrgLink + "/settings/actions/runners")
}
