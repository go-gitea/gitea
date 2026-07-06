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
	"gitea.dev/modules/web"
	actions_service "gitea.dev/services/actions"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
	secret_service "gitea.dev/services/secrets"
)

const (
	tplEnvironments    templates.TplName = "repo/settings/environments"
	tplEnvironmentEdit templates.TplName = "repo/settings/environment_edit"
)

// Environments renders the environment list page
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
	ctx.HTML(http.StatusOK, tplEnvironments)
}

// EnvironmentCreate handles POST to create a new environment
func EnvironmentCreate(ctx *context.Context) {
	name := strings.TrimSpace(ctx.FormString("name"))
	protectedBranches := strings.TrimSpace(ctx.FormString("protected_branches"))

	if name == "" {
		ctx.Flash.Error(ctx.Tr("environments.creation.failed"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/environments")
		return
	}

	_, err := actions_service.CreateEnvironment(ctx, ctx.Repo.Repository.ID, name, protectedBranches)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Flash.Error(err.Error())
		} else {
			ctx.Flash.Error(ctx.Tr("environments.creation.failed"))
		}
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/environments")
		return
	}

	ctx.Flash.Success(ctx.Tr("environments.creation.success", name))
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/environments/" + url.PathEscape(name))
}

// EnvironmentEdit renders the environment edit page (secrets + variables)
func EnvironmentEdit(ctx *context.Context) {
	envName := ctx.PathParam("environment_name")
	env, err := actions_model.GetEnvironmentByRepoAndName(ctx, ctx.Repo.Repository.ID, envName)
	if err != nil {
		ctx.NotFound(err)
		return
	}

	secrets, err := db.Find[secret_model.Secret](ctx, secret_model.FindSecretsOptions{
		RepoID:        ctx.Repo.Repository.ID,
		EnvironmentID: env.ID,
	})
	if err != nil {
		ctx.ServerError("FindEnvSecrets", err)
		return
	}

	variables, err := db.Find[actions_model.ActionVariable](ctx, actions_model.FindVariablesOpts{
		RepoID:        ctx.Repo.Repository.ID,
		EnvironmentID: env.ID,
	})
	if err != nil {
		ctx.ServerError("FindEnvVariables", err)
		return
	}

	ctx.Data["Title"] = env.Name
	ctx.Data["PageIsRepoSettingsEnvironments"] = true
	ctx.Data["Environment"] = env
	ctx.Data["Secrets"] = secrets
	ctx.Data["Variables"] = variables
	ctx.Data["DataMaxLength"] = secret_model.SecretDataMaxLength
	ctx.Data["DescriptionMaxLength"] = secret_model.SecretDescriptionMaxLength
	ctx.Data["Link"] = ctx.Repo.RepoLink + "/settings/environments/" + url.PathEscape(envName)
	ctx.HTML(http.StatusOK, tplEnvironmentEdit)
}

// EnvironmentUpdate handles POST to update environment settings
func EnvironmentUpdate(ctx *context.Context) {
	envName := ctx.PathParam("environment_name")
	env, err := actions_model.GetEnvironmentByRepoAndName(ctx, ctx.Repo.Repository.ID, envName)
	if err != nil {
		ctx.NotFound(err)
		return
	}

	protectedBranches := strings.TrimSpace(ctx.FormString("protected_branches"))
	_, err = actions_service.UpdateEnvironment(ctx, ctx.Repo.Repository.ID, env.ID, "", protectedBranches)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Flash.Error(err.Error())
		} else {
			ctx.Flash.Error(ctx.Tr("environments.update.failed"))
		}
	} else {
		ctx.Flash.Success(ctx.Tr("environments.update.success"))
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/environments/" + url.PathEscape(envName))
}

// EnvironmentDelete handles POST to delete an environment
func EnvironmentDelete(ctx *context.Context) {
	envName := ctx.PathParam("environment_name")
	env, err := actions_model.GetEnvironmentByRepoAndName(ctx, ctx.Repo.Repository.ID, envName)
	if err != nil {
		ctx.Flash.Error(ctx.Tr("environments.deletion.failed"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/environments")
		return
	}

	if err := actions_service.DeleteEnvironment(ctx, ctx.Repo.Repository.ID, env.ID); err != nil {
		ctx.Flash.Error(ctx.Tr("environments.deletion.failed"))
	} else {
		ctx.Flash.Success(ctx.Tr("environments.deletion.success"))
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/environments")
}

// EnvironmentSecretPost handles POST for adding/updating an environment secret
func EnvironmentSecretPost(ctx *context.Context) {
	envName := ctx.PathParam("environment_name")
	env, err := actions_model.GetEnvironmentByRepoAndName(ctx, ctx.Repo.Repository.ID, envName)
	if err != nil {
		ctx.NotFound(err)
		return
	}

	redirectURL := ctx.Repo.RepoLink + "/settings/environments/" + url.PathEscape(envName)
	form := web.GetForm(ctx).(*forms.AddSecretForm)

	if err := secret_service.ValidateName(form.Name); err != nil {
		ctx.Flash.Error(err.Error())
		ctx.Redirect(redirectURL)
		return
	}

	_, _, err = actions_service.CreateOrUpdateEnvSecret(ctx, ctx.Repo.Repository.ID, env.ID, form.Name, util.NormalizeStringEOL(form.Data), form.Description)
	if err != nil {
		ctx.Flash.Error(ctx.Tr("secrets.save_failed"))
	} else {
		ctx.Flash.Success(ctx.Tr("secrets.save_success", strings.ToUpper(form.Name)))
	}
	ctx.JSONRedirect(redirectURL)
}

// EnvironmentSecretDelete handles POST for deleting an environment secret
func EnvironmentSecretDelete(ctx *context.Context) {
	envName := ctx.PathParam("environment_name")
	env, err := actions_model.GetEnvironmentByRepoAndName(ctx, ctx.Repo.Repository.ID, envName)
	if err != nil {
		ctx.JSONError(ctx.Tr("secrets.deletion.failed"))
		return
	}

	id := ctx.FormInt64("id")
	secrets, err := db.Find[secret_model.Secret](ctx, secret_model.FindSecretsOptions{
		RepoID:        ctx.Repo.Repository.ID,
		EnvironmentID: env.ID,
		SecretID:      id,
	})
	if err != nil || len(secrets) == 0 {
		ctx.JSONError(ctx.Tr("secrets.deletion.failed"))
		return
	}

	if err := actions_service.DeleteEnvSecret(ctx, ctx.Repo.Repository.ID, env.ID, secrets[0].Name); err != nil {
		ctx.JSONError(ctx.Tr("secrets.deletion.failed"))
		return
	}

	ctx.Flash.Success(ctx.Tr("secrets.deletion.success"))
	ctx.JSONRedirect(ctx.Repo.RepoLink + "/settings/environments/" + url.PathEscape(envName))
}

// EnvironmentVariableCreate handles POST for creating an environment variable
func EnvironmentVariableCreate(ctx *context.Context) {
	envName := ctx.PathParam("environment_name")
	env, err := actions_model.GetEnvironmentByRepoAndName(ctx, ctx.Repo.Repository.ID, envName)
	if err != nil {
		ctx.NotFound(err)
		return
	}

	redirectURL := ctx.Repo.RepoLink + "/settings/environments/" + url.PathEscape(envName)
	form := web.GetForm(ctx).(*forms.EditVariableForm)

	_, err = actions_service.CreateEnvVariable(ctx, ctx.Repo.Repository.ID, env.ID, form.Name, form.Data, form.Description)
	if err != nil {
		ctx.Flash.Error(ctx.Tr("actions.variables.creation.failed"))
	} else {
		ctx.Flash.Success(ctx.Tr("actions.variables.creation.success", strings.ToUpper(form.Name)))
	}
	ctx.JSONRedirect(redirectURL)
}

// EnvironmentVariableUpdate handles POST for updating an environment variable
func EnvironmentVariableUpdate(ctx *context.Context) {
	envName := ctx.PathParam("environment_name")
	env, err := actions_model.GetEnvironmentByRepoAndName(ctx, ctx.Repo.Repository.ID, envName)
	if err != nil {
		ctx.NotFound(err)
		return
	}

	redirectURL := ctx.Repo.RepoLink + "/settings/environments/" + url.PathEscape(envName)
	variableID := ctx.PathParamInt64("variable_id")
	form := web.GetForm(ctx).(*forms.EditVariableForm)

	_, err = actions_service.UpdateEnvVariable(ctx, ctx.Repo.Repository.ID, env.ID, variableID, form.Name, form.Data, form.Description)
	if err != nil {
		ctx.Flash.Error(ctx.Tr("actions.variables.edit"))
	} else {
		ctx.Flash.Success(ctx.Tr("actions.variables.edit"))
	}
	ctx.JSONRedirect(redirectURL)
}

// EnvironmentVariableDelete handles POST for deleting an environment variable
func EnvironmentVariableDelete(ctx *context.Context) {
	envName := ctx.PathParam("environment_name")
	env, err := actions_model.GetEnvironmentByRepoAndName(ctx, ctx.Repo.Repository.ID, envName)
	if err != nil {
		ctx.JSONError(ctx.Tr("actions.variables.deletion.failed"))
		return
	}

	variableID := ctx.PathParamInt64("variable_id")
	if err := actions_service.DeleteEnvVariable(ctx, ctx.Repo.Repository.ID, env.ID, variableID); err != nil {
		ctx.JSONError(ctx.Tr("actions.variables.deletion.failed"))
		return
	}

	ctx.Flash.Success(ctx.Tr("actions.variables.deletion.success"))
	ctx.JSONRedirect(ctx.Repo.RepoLink + "/settings/environments/" + url.PathEscape(envName))
}
