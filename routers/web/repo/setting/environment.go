// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"net/http"

	actions_model "gitea.dev/models/actions"
	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	"gitea.dev/modules/log"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	shared_secrets "gitea.dev/routers/web/shared/secrets"
	actions_service "gitea.dev/services/actions"
	"gitea.dev/services/audit"
	"gitea.dev/services/context"
)

const (
	tplEnvironments    templates.TplName = "repo/settings/environments"
	tplEnvironmentEdit templates.TplName = "repo/settings/environment_edit"
)

func contextEnvironment(ctx *context.Context) *actions_model.ActionEnvironment {
	env, ok := ctx.Data["Environment"].(*actions_model.ActionEnvironment)
	if !ok {
		panic("EnvironmentAssignment must run before this handler")
	}
	return env
}

func EnvironmentAssignment(ctx *context.Context) {
	env, err := actions_model.GetEnvironmentByRepoAndName(ctx, ctx.Repo.Repository.ID, ctx.PathParam("environment_name"))
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("GetEnvironmentByRepoAndName", err)
		}
		return
	}
	ctx.Data["Environment"] = env
}

func Environments(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("environments.environments")
	ctx.Data["PageIsRepoSettingsEnvironments"] = true

	envs, err := db.Find[actions_model.ActionEnvironment](ctx, actions_model.FindEnvironmentsOptions{
		RepoID: ctx.Repo.Repository.ID,
	})
	if err != nil {
		ctx.ServerError("FindEnvironments", err)
		return
	}
	ctx.Data["Environments"] = envs
	ctx.Data["EnvironmentNameMaxLength"] = actions_model.EnvironmentNameMaxLength
	ctx.HTML(http.StatusOK, tplEnvironments)
}

func EnvironmentCreate(ctx *context.Context) {
	env, created, err := actions_service.GetOrCreateEnvironment(ctx, ctx.Repo.Repository.ID, ctx.FormString("name"), formBranchPatterns(ctx))
	if err == nil && !created {
		err = util.ErrorWrapTranslatable(util.NewAlreadyExistErrorf("environment %q already exists", env.Name), "environments.name_already_exists", env.Name)
	}
	if err != nil {
		flashEnvironmentError(ctx, err)
		ctx.Redirect(ctx.Link)
		return
	}

	audit.Record(ctx, audit_model.RepositoryEnvironmentAdd, ctx.Repo.Repository, "environment", env.Name)
	ctx.Flash.Success(ctx.Tr("environments.creation.success", env.Name))
	ctx.Redirect(env.SettingsLink(ctx.Repo.RepoLink))
}

func EnvironmentEdit(ctx *context.Context) {
	env := contextEnvironment(ctx)
	ctx.Data["Title"] = env.Name
	ctx.Data["PageIsRepoSettingsEnvironments"] = true

	variables, err := db.Find[actions_model.ActionVariable](ctx, actions_model.FindVariablesOpts{
		RepoID:        ctx.Repo.Repository.ID,
		EnvironmentID: env.ID,
	})
	if err != nil {
		ctx.ServerError("FindVariables", err)
		return
	}
	ctx.Data["Variables"] = variables

	shared_secrets.SetSecretsContext(ctx, 0, ctx.Repo.Repository.ID, env.ID)
	if ctx.Written() {
		return
	}
	ctx.HTML(http.StatusOK, tplEnvironmentEdit)
}

func EnvironmentUpdate(ctx *context.Context) {
	env := contextEnvironment(ctx)
	if changed, err := actions_service.UpdateEnvironment(ctx, env, formBranchPatterns(ctx)); err != nil {
		flashEnvironmentError(ctx, err)
	} else {
		if changed {
			audit.Record(ctx, audit_model.RepositoryEnvironmentUpdate, ctx.Repo.Repository, "environment", env.Name)
		}
		ctx.Flash.Success(ctx.Tr("environments.update.success"))
	}
	ctx.Redirect(ctx.Link)
}

func EnvironmentDelete(ctx *context.Context) {
	env := contextEnvironment(ctx)
	if err := actions_model.DeleteEnvironment(ctx, ctx.Repo.Repository.ID, env.ID); err != nil {
		flashEnvironmentError(ctx, err)
	} else {
		audit.Record(ctx, audit_model.RepositoryEnvironmentRemove, ctx.Repo.Repository, "environment", env.Name)
		ctx.Flash.Success(ctx.Tr("environments.deletion.success"))
	}
	ctx.JSONRedirect(ctx.Repo.RepoLink + "/settings/actions/environments")
}

func formBranchPatterns(ctx *context.Context) []string {
	return actions_model.SplitBranchPatterns(ctx.FormString("allowed_branch_patterns"))
}

func flashEnvironmentError(ctx *context.Context, err error) {
	if translatable := util.ErrorAsTranslatable(err); translatable != nil {
		ctx.Flash.Error(translatable.Translate(ctx.Locale))
		return
	}
	log.Error("Environment settings failed: %v", err)
	ctx.Flash.Error(ctx.Tr("error.occurred"))
}
