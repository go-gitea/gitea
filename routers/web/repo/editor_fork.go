// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
	repo_service "code.gitea.io/gitea/services/repository"
)

const tplEditorFork templates.TplName = "repo/editor/fork"

func ForkToEdit(ctx *context.Context) {
	ctx.HTML(http.StatusOK, tplEditorFork)
}

func ForkToEditPost(ctx *context.Context) {
	ForkRepoTo(ctx, ctx.Doer, repo_service.ForkRepoOptions{
		BaseRepo:     ctx.Repo.Repository,
		Name:         getUniqueRepositoryName(ctx, ctx.Doer.ID, ctx.Repo.Repository.Name),
		Description:  ctx.Repo.Repository.Description,
		SingleBranch: ctx.Repo.Repository.DefaultBranch, // maybe we only need the default branch in the fork?
	})
	if ctx.Written() {
		return
	}
	ctx.JSONRedirect("") // reload the page, the new fork should be editable now
}
