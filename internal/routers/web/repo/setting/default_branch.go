// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"

	repo_model "code.gitea.io/gitea/internal/models/repo"
	"code.gitea.io/gitea/internal/modules/context"
	"code.gitea.io/gitea/internal/modules/git"
	"code.gitea.io/gitea/internal/modules/log"
	"code.gitea.io/gitea/internal/modules/setting"
	"code.gitea.io/gitea/internal/routers/web/repo"
	notify_service "code.gitea.io/gitea/internal/services/notify"
)

// SetDefaultBranchPost set default branch
func SetDefaultBranchPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.branches.update_default_branch")
	ctx.Data["PageIsSettingsBranches"] = true

	repo.PrepareBranchList(ctx)
	if ctx.Written() {
		return
	}

	repo := ctx.Repo.Repository

	switch ctx.FormString("action") {
	case "default_branch":
		if ctx.HasError() {
			ctx.HTML(http.StatusOK, tplBranches)
			return
		}

		branch := ctx.FormString("branch")
		if !ctx.Repo.GitRepo.IsBranchExist(branch) {
			ctx.Status(http.StatusNotFound)
			return
		} else if repo.DefaultBranch != branch {
			repo.DefaultBranch = branch
			if err := ctx.Repo.GitRepo.SetDefaultBranch(branch); err != nil {
				if !git.IsErrUnsupportedVersion(err) {
					ctx.ServerError("SetDefaultBranch", err)
					return
				}
			}
			if err := repo_model.UpdateDefaultBranch(ctx, repo); err != nil {
				ctx.ServerError("SetDefaultBranch", err)
				return
			}

			notify_service.ChangeDefaultBranch(ctx, repo)
		}

		log.Trace("Repository basic settings updated: %s/%s", ctx.Repo.Owner.Name, repo.Name)

		ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
		ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
	default:
		ctx.NotFound("", nil)
	}
}
