// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"
	"strconv"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
)

// EnvironmentsPost handles the creation and deletion of environments
func EnvironmentsPost(ctx *context.Context) {
	if !setting.Actions.Enabled {
		ctx.NotFound(util.NewNotExistErrorf("Actions not enabled"))
		return
	}

	form := web.GetForm(ctx).(*forms.AddSecretForm)

	switch ctx.FormString("action") {
	case "add":
		if ctx.HasError() {
			ctx.JSONError(ctx.GetErrMsg())
			return
		}

		envOpts := actions_model.CreateEnvironmentOptions{
			RepoID:      ctx.Repo.Repository.ID,
			Name:        form.Name,
			Description: form.Data, // Reusing the data field for description
			CreatedByID: ctx.Doer.ID,
		}

		if _, err := actions_model.CreateEnvironment(ctx, envOpts); err != nil {
			ctx.JSONError(err.Error())
			return
		}

		ctx.JSONRedirect(ctx.Repo.RepoLink + "/settings/actions/environments")
	case "remove":
		id, _ := strconv.ParseInt(ctx.FormString("id"), 10, 64)
		env, err := actions_model.GetEnvironmentByRepoIDAndName(ctx, ctx.Repo.Repository.ID, ctx.FormString("name"))
		if err != nil {
			ctx.JSONError(err.Error())
			return
		}

		if env.ID != id {
			ctx.JSONError("Environment not found")
			return
		}

		if err := actions_model.DeleteEnvironment(ctx, ctx.Repo.Repository.ID, env.Name); err != nil {
			ctx.JSONError(err.Error())
			return
		}

		ctx.JSONRedirect(ctx.Repo.RepoLink + "/settings/actions/environments")
	}
}

// Environments displays the environment management page
func Environments(ctx *context.Context) {
	if !setting.Actions.Enabled {
		ctx.NotFound(util.NewNotExistErrorf("Actions not enabled"))
		return
	}

	ctx.Data["Title"] = ctx.Tr("actions.environments")
	ctx.Data["PageIsRepoSettings"] = true
	ctx.Data["PageIsRepoSettingsActions"] = true
	ctx.Data["PageIsSharedSettingsEnvironments"] = true

	envs, err := actions_model.FindEnvironments(ctx, actions_model.FindEnvironmentsOptions{
		RepoID: ctx.Repo.Repository.ID,
	})
	if err != nil {
		ctx.ServerError("FindEnvironments", err)
		return
	}

	ctx.Data["Environments"] = envs
	ctx.Data["PageType"] = "environment"

	ctx.HTML(http.StatusOK, "repo/settings/actions")
}