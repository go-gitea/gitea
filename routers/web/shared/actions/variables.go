// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/web"
	actions_service "code.gitea.io/gitea/services/actions"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
)

func SetVariablesContext(ctx *context.Context, ownerID, repoID int64) {
	variables, err := db.Find[actions_model.ActionVariable](ctx, actions_model.FindVariablesOpts{
		OwnerID: ownerID,
		RepoID:  repoID,
	})
	if err != nil {
		ctx.ServerError("FindVariables", err)
		return
	}
	ctx.Data["Variables"] = variables
}

func CreateVariable(ctx *context.Context, ownerID, repoID int64, redirectURL string) {
	form := web.GetForm(ctx).(*forms.EditVariableForm)

	v, err := actions_service.CreateVariable(ctx, ownerID, repoID, form.Name, form.Data)
	if err != nil {
		log.Error("CreateVariable: %v", err)
		ctx.JSONError(ctx.Tr("actions.variables.creation.failed"))
		return
	}

	ctx.Flash.Success(ctx.Tr("actions.variables.creation.success", v.Name))
	ctx.JSONRedirect(redirectURL)
}

func UpdateVariable(ctx *context.Context, redirectURL string) {
	id := ctx.PathParamInt64("variable_id")
	form := web.GetForm(ctx).(*forms.EditVariableForm)

	if ok, err := actions_service.UpdateVariable(ctx, id, form.Name, form.Data); err != nil || !ok {
		log.Error("UpdateVariable: %v", err)
		ctx.JSONError(ctx.Tr("actions.variables.update.failed"))
		return
	}
	ctx.Flash.Success(ctx.Tr("actions.variables.update.success"))
	ctx.JSONRedirect(redirectURL)
}

func DeleteVariable(ctx *context.Context, redirectURL string) {
	id := ctx.PathParamInt64("variable_id")

	if err := actions_service.DeleteVariableByID(ctx, id); err != nil {
		log.Error("Delete variable [%d] failed: %v", id, err)
		ctx.JSONError(ctx.Tr("actions.variables.deletion.failed"))
		return
	}
	ctx.Flash.Success(ctx.Tr("actions.variables.deletion.success"))
	ctx.JSONRedirect(redirectURL)
}
