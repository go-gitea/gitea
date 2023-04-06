// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/url"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/context"
	actions_shared "code.gitea.io/gitea/routers/web/shared/actions"
)

// Runners render runners page
func Runners(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("org.runners")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsOrgSettingsRunners"] = true

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

	actions_shared.RunnersList(ctx, tplSettingsRunners, opts)
}

// ResetRunnerRegistrationToken reset runner registration token
func ResetRunnerRegistrationToken(ctx *context.Context) {
	actions_shared.RunnerResetRegistrationToken(ctx,
		ctx.Org.Organization.ID, 0,
		ctx.Org.OrgLink+"/settings/runners")
}

// RunnersEdit render runner edit page
func RunnersEdit(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("org.runners.edit")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsOrgSettingsRunners"] = true
	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	actions_shared.RunnerDetails(ctx, tplSettingsRunnersEdit, page,
		ctx.ParamsInt64(":runnerid"), ctx.Org.Organization.ID, 0,
	)
}

// RunnersEditPost response for editing runner
func RunnersEditPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("org.runners.edit")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsOrgSettingsRunners"] = true
	actions_shared.RunnerDetailsEditPost(ctx, ctx.ParamsInt64(":runnerid"),
		ctx.Org.Organization.ID, 0,
		ctx.Org.OrgLink+"/settings/runners/"+url.PathEscape(ctx.Params(":runnerid")))
}

// RunnerDeletePost response for deleting runner
func RunnerDeletePost(ctx *context.Context) {
	actions_shared.RunnerDeletePost(ctx,
		ctx.ParamsInt64(":runnerid"),
		ctx.Org.OrgLink+"/settings/runners",
		ctx.Org.OrgLink+"/settings/runners/"+url.PathEscape(ctx.Params(":runnerid")))
}
