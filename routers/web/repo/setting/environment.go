// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	shared_actions "gitea.dev/routers/web/shared/actions"
	shared_secrets "gitea.dev/routers/web/shared/secrets"
	actions_service "gitea.dev/services/actions"
	"gitea.dev/services/context"
)

const (
	tplEnvironments    templates.TplName = "repo/settings/environments"
	tplEnvironmentEdit templates.TplName = "repo/settings/environment_edit"
)

func Environments(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("environments.environments")
	ctx.Data["PageIsRepoSettingsEnvironments"] = true

	envs, err := db.Find[actions_model.ActionEnvironment](ctx, actions_model.FindEnvironmentsOptions{
		RepoID: ctx.Repo.Repository.ID,
	})
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	ctx.Data["Environments"] = envs
	ctx.Data["EnvironmentNameMaxLength"] = actions_model.EnvironmentNameMaxLength
	ctx.HTML(http.StatusOK, tplEnvironments)
}

func EnvironmentCreate(ctx *context.Context) {
	env, created, err := actions_service.GetOrCreateEnvironment(ctx, ctx.Repo.Repository, ctx.FormString("name"), formBranchPatterns(ctx))
	if err == nil && !created {
		err = util.ErrorWrapTranslatable(util.NewAlreadyExistErrorf("environment %q already exists", env.Name), "environments.name_already_exists", env.Name)
	}
	if err != nil {
		ctx.JSONErrorAuto(err)
		return
	}
	ctx.Flash.Success(ctx.Tr("environments.creation.success", env.Name))
	ctx.JSONRedirect(env.SettingsLink(ctx.Repo.RepoLink))
}

func EnvironmentEdit(ctx *context.Context) {
	env := shared_actions.EnvironmentFromContext(ctx)
	ctx.Data["Title"] = env.Name
	ctx.Data["PageIsRepoSettingsEnvironments"] = true

	scope := env.Scope()
	variables, err := db.Find[actions_model.ActionVariable](ctx, actions_model.FindVariablesOpts{BelongingScope: scope})
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	ctx.Data["Variables"] = variables
	ctx.Data["VariableDataMaxLength"] = actions_model.VariableDataMaxLength
	ctx.Data["VariableDescriptionMaxLength"] = actions_model.VariableDescriptionMaxLength

	shared_secrets.SetSecretsContext(ctx, scope)
	if ctx.Written() {
		return
	}
	ctx.HTML(http.StatusOK, tplEnvironmentEdit)
}

func EnvironmentUpdate(ctx *context.Context) {
	if err := actions_service.UpdateEnvironment(ctx, ctx.Repo.Repository, shared_actions.EnvironmentFromContext(ctx), formBranchPatterns(ctx)); err != nil {
		ctx.JSONErrorAuto(err)
		return
	}
	ctx.Flash.Success(ctx.Tr("environments.update.success"))
	ctx.JSONRedirect(ctx.Link)
}

func EnvironmentDelete(ctx *context.Context) {
	if err := actions_service.DeleteEnvironment(ctx, ctx.Repo.Repository, shared_actions.EnvironmentFromContext(ctx)); err != nil {
		ctx.JSONErrorAuto(err)
		return
	}
	ctx.Flash.Success(ctx.Tr("environments.deletion.success"))
	ctx.JSONRedirect(ctx.Repo.RepoLink + "/settings/actions/environments")
}

// formBranchPatterns leaves the textarea to JoinBranchPatterns, which splits it into lines
func formBranchPatterns(ctx *context.Context) []string {
	return []string{ctx.FormString("allowed_branch_patterns")}
}
