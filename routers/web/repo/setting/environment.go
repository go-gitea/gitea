// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	secret_model "gitea.dev/models/secret"
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

func environmentsLink(ctx *context.Context) string {
	return ctx.Repo.RepoLink + "/settings/actions/environments"
}

func environmentLink(ctx *context.Context, env *actions_model.ActionEnvironment) string {
	return environmentsLink(ctx) + "/" + url.PathEscape(env.Name)
}

// contextEnvironment returns the environment EnvironmentAssignment put on the request.
func contextEnvironment(ctx *context.Context) *actions_model.ActionEnvironment {
	env, ok := ctx.Data["Environment"].(*actions_model.ActionEnvironment)
	if !ok {
		panic("EnvironmentAssignment must run before this handler")
	}
	return env
}

// EnvironmentAssignment loads the environment named in the route for the handlers below it.
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
	ctx.Data["Link"] = environmentLink(ctx, env)
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
	ctx.Data["Link"] = environmentsLink(ctx)
	ctx.HTML(http.StatusOK, tplEnvironments)
}

func EnvironmentCreate(ctx *context.Context) {
	name := strings.TrimSpace(ctx.FormString("name"))
	env, err := actions_service.CreateEnvironment(ctx, ctx.Repo.Repository.ID, name, formBranchPatterns(ctx))
	if err != nil {
		flashEnvironmentError(ctx, err, "environments.creation.failed")
		ctx.Redirect(environmentsLink(ctx))
		return
	}

	ctx.Flash.Success(ctx.Tr("environments.creation.success", env.Name))
	ctx.Redirect(environmentLink(ctx, env))
}

func EnvironmentEdit(ctx *context.Context) {
	env := contextEnvironment(ctx)
	ctx.Data["Title"] = env.Name
	ctx.Data["PageIsRepoSettingsEnvironments"] = true

	secrets, err := db.Find[secret_model.Secret](ctx, secret_model.FindSecretsOptions{
		RepoID:        ctx.Repo.Repository.ID,
		EnvironmentID: env.ID,
	})
	if err != nil {
		ctx.ServerError("FindSecrets", err)
		return
	}
	variables, err := db.Find[actions_model.ActionVariable](ctx, actions_model.FindVariablesOpts{
		RepoID:        ctx.Repo.Repository.ID,
		EnvironmentID: env.ID,
	})
	if err != nil {
		ctx.ServerError("FindVariables", err)
		return
	}

	ctx.Data["Secrets"] = secrets
	ctx.Data["Variables"] = variables
	ctx.Data["SecretDataMaxLength"] = secret_model.SecretDataMaxLength
	ctx.Data["SecretDescriptionMaxLength"] = secret_model.SecretDescriptionMaxLength
	ctx.Data["VariableDataMaxLength"] = actions_model.VariableDataMaxLength
	ctx.Data["VariableDescriptionMaxLength"] = actions_model.VariableDescriptionMaxLength
	ctx.HTML(http.StatusOK, tplEnvironmentEdit)
}

func EnvironmentUpdate(ctx *context.Context) {
	env := contextEnvironment(ctx)
	if err := actions_service.UpdateEnvironment(ctx, env, env.Name, formBranchPatterns(ctx)); err != nil {
		flashEnvironmentError(ctx, err, "environments.update.failed")
	} else {
		ctx.Flash.Success(ctx.Tr("environments.update.success"))
	}
	ctx.Redirect(environmentLink(ctx, env))
}

func EnvironmentDelete(ctx *context.Context) {
	env := contextEnvironment(ctx)
	if err := actions_model.DeleteEnvironment(ctx, ctx.Repo.Repository.ID, env.ID); err != nil {
		ctx.Flash.Error(ctx.Tr("environments.deletion.failed"))
	} else {
		ctx.Flash.Success(ctx.Tr("environments.deletion.success"))
	}
	ctx.JSONRedirect(environmentsLink(ctx))
}

func EnvironmentSecretPost(ctx *context.Context) {
	env := contextEnvironment(ctx)
	shared_secrets.PerformSecretsPost(ctx, 0, ctx.Repo.Repository.ID, env.ID, environmentLink(ctx, env))
}

func EnvironmentSecretDelete(ctx *context.Context) {
	env := contextEnvironment(ctx)
	shared_secrets.PerformSecretsDelete(ctx, 0, ctx.Repo.Repository.ID, env.ID, environmentLink(ctx, env))
}

func EnvironmentVariableCreate(ctx *context.Context) {
	env := contextEnvironment(ctx)
	shared_actions.PerformEnvVariableCreate(ctx, ctx.Repo.Repository.ID, env.ID, environmentLink(ctx, env))
}

func EnvironmentVariableUpdate(ctx *context.Context) {
	env := contextEnvironment(ctx)
	shared_actions.PerformEnvVariableUpdate(ctx, ctx.Repo.Repository.ID, env.ID, environmentLink(ctx, env))
}

func EnvironmentVariableDelete(ctx *context.Context) {
	env := contextEnvironment(ctx)
	shared_actions.PerformEnvVariableDelete(ctx, ctx.Repo.Repository.ID, env.ID, environmentLink(ctx, env))
}

// formBranchPatterns reads the textarea holding one glob per line.
func formBranchPatterns(ctx *context.Context) []string {
	return actions_model.SplitBranchPatterns(ctx.FormString("allowed_branch_patterns"))
}

func flashEnvironmentError(ctx *context.Context, err error, fallbackKey string) {
	var errName actions_model.ErrInvalidEnvironmentName
	var errPattern actions_model.ErrInvalidBranchPattern
	var errExists actions_model.ErrEnvironmentAlreadyExists
	switch {
	case errors.As(err, &errName):
		ctx.Flash.Error(ctx.Tr("environments.name_invalid", actions_model.EnvironmentNameMaxLength))
	case errors.As(err, &errPattern):
		ctx.Flash.Error(ctx.Tr("environments.branch_pattern_invalid", errPattern.Pattern))
	case errors.As(err, &errExists):
		ctx.Flash.Error(ctx.Tr("environments.name_already_exists", errExists.Name))
	default:
		ctx.Flash.Error(ctx.Tr(fallbackKey))
	}
}
