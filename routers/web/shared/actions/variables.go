// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"regexp"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
	secret_service "code.gitea.io/gitea/services/secrets"
)

func SetVariablesContext(ctx *context.Context, ownerID, repoID int64) {
	variables, err := actions_model.FindVariables(ctx, actions_model.FindVariablesOpts{
		OwnerID: ownerID,
		RepoID:  repoID,
	})
	if err != nil {
		ctx.ServerError("FindVariables", err)
		return
	}
	ctx.Data["Variables"] = variables
}

// some regular expression of `variables` and `secrets`
// reference to:
// https://docs.github.com/en/actions/learn-github-actions/variables#naming-conventions-for-configuration-variables
// https://docs.github.com/en/actions/security-guides/encrypted-secrets#naming-your-secrets
var (
	forbiddenEnvNameCIRx = regexp.MustCompile("(?i)^CI")
)

func envNameCIRegexMatch(name string) error {
	if forbiddenEnvNameCIRx.MatchString(name) {
		log.Error("Env Name cannot be ci")
		return errors.New("env name cannot be ci")
	}
	return nil
}

func CreateVariable(ctx *context.Context, ownerID, repoID int64, redirectURL string) {
	form := web.GetForm(ctx).(*forms.EditVariableForm)

	if err := secret_service.ValidateName(form.Name); err != nil {
		ctx.JSONError(err.Error())
		return
	}

	if err := envNameCIRegexMatch(form.Name); err != nil {
		ctx.JSONError(err.Error())
		return
	}

	v, err := actions_model.InsertVariable(ctx, ownerID, repoID, form.Name, ReserveLineBreakForTextarea(form.Data))
	if err != nil {
		log.Error("InsertVariable error: %v", err)
		ctx.JSONError(ctx.Tr("actions.variables.creation.failed"))
		return
	}
	ctx.Flash.Success(ctx.Tr("actions.variables.creation.success", v.Name))
	ctx.JSONRedirect(redirectURL)
}

func UpdateVariable(ctx *context.Context, redirectURL string) {
	id := ctx.ParamsInt64(":variable_id")
	form := web.GetForm(ctx).(*forms.EditVariableForm)

	if err := secret_service.ValidateName(form.Name); err != nil {
		ctx.JSONError(err.Error())
		return
	}

	if err := envNameCIRegexMatch(form.Name); err != nil {
		ctx.JSONError(err.Error())
		return
	}

	ok, err := actions_model.UpdateVariable(ctx, &actions_model.ActionVariable{
		ID:   id,
		Name: strings.ToUpper(form.Name),
		Data: ReserveLineBreakForTextarea(form.Data),
	})
	if err != nil || !ok {
		log.Error("UpdateVariable error: %v", err)
		ctx.JSONError(ctx.Tr("actions.variables.update.failed"))
		return
	}
	ctx.Flash.Success(ctx.Tr("actions.variables.update.success"))
	ctx.JSONRedirect(redirectURL)
}

func DeleteVariable(ctx *context.Context, redirectURL string) {
	id := ctx.ParamsInt64(":variable_id")

	if _, err := db.DeleteByBean(ctx, &actions_model.ActionVariable{ID: id}); err != nil {
		log.Error("Delete variable [%d] failed: %v", id, err)
		ctx.JSONError(ctx.Tr("actions.variables.deletion.failed"))
		return
	}
	ctx.Flash.Success(ctx.Tr("actions.variables.deletion.success"))
	ctx.JSONRedirect(redirectURL)
}

func ReserveLineBreakForTextarea(input string) string {
	// Since the content is from a form which is a textarea, the line endings are \r\n.
	// It's a standard behavior of HTML.
	// But we want to store them as \n like what GitHub does.
	// And users are unlikely to really need to keep the \r.
	// Other than this, we should respect the original content, even leading or trailing spaces.
	return strings.ReplaceAll(input, "\r\n", "\n")
}
