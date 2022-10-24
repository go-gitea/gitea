package org

import (
	"net/url"

	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/routers/common"
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

	opts := bots_model.FindRunnerOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: 100,
		},
		Sort:        ctx.Req.URL.Query().Get("sort"),
		Filter:      ctx.Req.URL.Query().Get("q"),
		WithDeleted: false,
		RepoID:      0,
		OwnerID:     ctx.Org.Organization.ID,
	}

	common.RunnersList(ctx, tplSettingsRunners, opts)
}

// ResetRunnerRegistrationToken reset runner registration token
func ResetRunnerRegistrationToken(ctx *context.Context) {
	common.RunnerResetRegistrationToken(ctx,
		ctx.Org.Organization.ID, 0,
		ctx.Org.OrgLink+"/settings/runners")
}

// RunnersEdit render runner edit page
func RunnersEdit(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("org.runners.edit")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsOrgSettingsRunners"] = true

	common.RunnerDetails(ctx, tplSettingsRunnersEdit,
		ctx.ParamsInt64(":runnerid"), ctx.Org.Organization.ID, 0,
	)
}

// RunnersEditPost response for editing runner
func RunnersEditPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("org.runners.edit")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsOrgSettingsRunners"] = true
	common.RunnerDetailsEditPost(ctx, ctx.ParamsInt64(":runnerid"),
		ctx.Org.Organization.ID, 0,
		ctx.Org.OrgLink+"/settings/runners/"+url.PathEscape(ctx.Params(":runnerid")))
}
