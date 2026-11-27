// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"net/http"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/modules/log"
	"gitea.dev/modules/web"
	actions_service "gitea.dev/services/actions"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
)

func Variables(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.variables")
	ctx.Data["PageType"] = "variables"
	ctx.Data["PageIsSharedSettingsVariables"] = true

	scope, err := getSettingsScope(ctx)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	variables, err := actions_model.FindVariables(ctx, actions_model.FindVariablesOpts{BelongingScope: scope.BelongingScope})
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	ctx.Data["Variables"] = variables
	ctx.Data["DataMaxLength"] = actions_model.VariableDataMaxLength
	ctx.Data["DescriptionMaxLength"] = actions_model.VariableDescriptionMaxLength
	ctx.HTML(http.StatusOK, scope.Template)
}

func VariableCreate(ctx *context.Context) {
	scope, err := getSettingsScope(ctx)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	form := web.GetForm[*forms.EditVariableForm](ctx)

	variable, err := actions_service.CreateVariable(ctx, scope.BelongingScope, form.Name, form.Data, form.Description)
	if err != nil {
		ctx.JSONErrorAuto(err)
		return
	}

	ctx.Flash.Success(ctx.Tr("actions.variables.creation.success", variable.Name))
	ctx.JSONRedirect(scope.LinkPrefix + "/variables")
}

func VariableUpdate(ctx *context.Context) {
	scope, err := getSettingsScope(ctx)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	variable := findActionsVariable(ctx, ctx.PathParamInt64("variable_id"), scope.BelongingScope)
	if ctx.Written() {
		return
	}

	form := web.GetForm[*forms.EditVariableForm](ctx)
	variable.Name = form.Name
	variable.Data = form.Data
	variable.Description = form.Description

	if _, err := actions_service.UpdateVariableNameData(ctx, variable); err != nil {
		ctx.JSONErrorAuto(err)
		return
	}
	ctx.Flash.Success(ctx.Tr("actions.variables.update.success"))
	ctx.JSONRedirect(scope.LinkPrefix + "/variables")
}

func findActionsVariable(ctx *context.Context, id int64, scope actions_model.BelongingScope) *actions_model.ActionVariable {
	variables, err := actions_model.FindVariables(ctx, actions_model.FindVariablesOpts{
		IDs:            []int64{id},
		BelongingScope: scope,
	})
	if err != nil {
		ctx.ServerErrorAuto(err)
		return nil
	} else if len(variables) == 0 {
		ctx.NotFound()
		return nil
	}
	return variables[0]
}

func VariableDelete(ctx *context.Context) {
	scope, err := getSettingsScope(ctx)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	id := ctx.PathParamInt64("variable_id")

	variable := findActionsVariable(ctx, id, scope.BelongingScope)
	if ctx.Written() {
		return
	}

	if err := actions_service.DeleteVariable(ctx, variable); err != nil {
		log.Error("Delete variable [%d] failed: %v", id, err)
		ctx.JSONError(ctx.Tr("actions.variables.deletion.failed"))
		return
	}
	ctx.Flash.Success(ctx.Tr("actions.variables.deletion.success"))
	ctx.JSONRedirect(scope.LinkPrefix + "/variables")
}
