// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"net/url"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/context"
	actions_shared "code.gitea.io/gitea/routers/web/shared/actions"
)

const (
	tplActions    = "repo/settings/actions"
	tplRunnerEdit = "repo/settings/runner_edit"
)

// Actions render settings/actions page for repo level
func Actions(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.actions")
	ctx.Data["PageIsSettingsActions"] = true

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
		RepoID:        ctx.Repo.Repository.ID,
		WithAvailable: true,
	}

	actions_shared.RunnersList(ctx, opts)
	GetSecrets(ctx)
	ctx.HTML(http.StatusOK, tplActions)
}

// RunnersEdit renders runner edit page for repository level
func RunnersEdit(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.runners")
	ctx.Data["PageIsSettingsRunners"] = true
	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	actions_shared.RunnerDetails(ctx, page,
		ctx.ParamsInt64(":runnerid"), 0, ctx.Repo.Repository.ID,
	)

	ctx.HTML(http.StatusOK, tplRunnerEdit)
}

func RunnersEditPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.runners")
	ctx.Data["PageIsSettingsRunners"] = true
	actions_shared.RunnerDetailsEditPost(ctx, ctx.ParamsInt64(":runnerid"),
		0, ctx.Repo.Repository.ID,
		ctx.Repo.RepoLink+"/settings/actions/runners/"+url.PathEscape(ctx.Params(":runnerid")))
}

func ResetRunnerRegistrationToken(ctx *context.Context) {
	actions_shared.RunnerResetRegistrationToken(ctx,
		0, ctx.Repo.Repository.ID,
		ctx.Repo.RepoLink+"/settings/actions")
}

// RunnerDeletePost response for deleting runner
func RunnerDeletePost(ctx *context.Context) {
	actions_shared.RunnerDeletePost(ctx, ctx.ParamsInt64(":runnerid"),
		ctx.Repo.RepoLink+"/settings/actions",
		ctx.Repo.RepoLink+"/settings/actions/runners/"+url.PathEscape(ctx.Params(":runnerid")))
}
