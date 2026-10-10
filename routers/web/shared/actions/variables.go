// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"net/http"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/modules/log"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/web"
	actions_service "gitea.dev/services/actions"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
)

const (
	tplRepoVariables  templates.TplName = "repo/settings/actions"
	tplOrgVariables   templates.TplName = "org/settings/actions"
	tplUserVariables  templates.TplName = "user/settings/actions"
	tplAdminVariables templates.TplName = "admin/actions"
)

type variablesCtx struct {
	*settingsScope
	VariablesTemplate templates.TplName
	RedirectLink      string
}

func getVariablesCtx(ctx *context.Context) (*variablesCtx, error) {
	scope, err := getSettingsScope(ctx)
	if err != nil {
		return nil, err
	}
	vCtx := &variablesCtx{settingsScope: scope, RedirectLink: scope.LinkPrefix + "/variables"}
	switch {
	case scope.IsRepo:
		vCtx.VariablesTemplate = tplRepoVariables
	case scope.IsOrg:
		vCtx.VariablesTemplate = tplOrgVariables
	case scope.IsUser:
		vCtx.VariablesTemplate = tplUserVariables
	case scope.IsAdmin:
		vCtx.VariablesTemplate = tplAdminVariables
	}
	return vCtx, nil
}

func Variables(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.variables")
	ctx.Data["PageType"] = "variables"
	ctx.Data["PageIsSharedSettingsVariables"] = true

	vCtx, err := getVariablesCtx(ctx)
	if err != nil {
		ctx.ServerError("getVariablesCtx", err)
		return
	}

	variables, err := db.Find[actions_model.ActionVariable](ctx, actions_model.FindVariablesOpts{
		OwnerID: vCtx.OwnerID,
		RepoID:  vCtx.RepoID,
	})
	if err != nil {
		ctx.ServerError("FindVariables", err)
		return
	}
	ctx.Data["Variables"] = variables
	ctx.Data["DataMaxLength"] = actions_model.VariableDataMaxLength
	ctx.Data["DescriptionMaxLength"] = actions_model.VariableDescriptionMaxLength
	ctx.HTML(http.StatusOK, vCtx.VariablesTemplate)
}

func VariableCreate(ctx *context.Context) {
	vCtx, err := getVariablesCtx(ctx)
	if err != nil {
		ctx.ServerError("getVariablesCtx", err)
		return
	}

	if ctx.HasError() { // form binding validation error
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	form := web.GetForm[*forms.EditVariableForm](ctx)

	v, err := actions_service.CreateVariable(ctx, vCtx.OwnerID, vCtx.RepoID, form.Name, form.Data, form.Description)
	if err != nil {
		ctx.JSONErrorAuto(err)
		return
	}

	ctx.Flash.Success(ctx.Tr("actions.variables.creation.success", v.Name))
	ctx.JSONRedirect(vCtx.RedirectLink)
}

func VariableUpdate(ctx *context.Context) {
	vCtx, err := getVariablesCtx(ctx)
	if err != nil {
		ctx.ServerError("getVariablesCtx", err)
		return
	}

	if ctx.HasError() { // form binding validation error
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	id := ctx.PathParamInt64("variable_id")

	variable := findActionsVariable(ctx, id, vCtx)
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
	ctx.JSONRedirect(vCtx.RedirectLink)
}

func findActionsVariable(ctx *context.Context, id int64, vCtx *variablesCtx) *actions_model.ActionVariable {
	opts := actions_model.FindVariablesOpts{
		IDs: []int64{id},
	}
	switch {
	case vCtx.IsRepo:
		opts.RepoID = vCtx.RepoID
		if opts.RepoID == 0 {
			panic("RepoID is 0")
		}
	case vCtx.IsOrg, vCtx.IsUser:
		opts.OwnerID = vCtx.OwnerID
		if opts.OwnerID == 0 {
			panic("OwnerID is 0")
		}
	case vCtx.IsAdmin:
		// do nothing
	default:
		panic("invalid actions variable")
	}

	got, err := actions_model.FindVariables(ctx, opts)
	if err != nil {
		ctx.ServerError("FindVariables", err)
		return nil
	} else if len(got) == 0 {
		ctx.NotFound(nil)
		return nil
	}
	return got[0]
}

func VariableDelete(ctx *context.Context) {
	vCtx, err := getVariablesCtx(ctx)
	if err != nil {
		ctx.ServerError("getVariablesCtx", err)
		return
	}

	id := ctx.PathParamInt64("variable_id")

	variable := findActionsVariable(ctx, id, vCtx)
	if ctx.Written() {
		return
	}

	if err := actions_service.DeleteVariableByID(ctx, variable.ID); err != nil {
		log.Error("Delete variable [%d] failed: %v", id, err)
		ctx.JSONError(ctx.Tr("actions.variables.deletion.failed"))
		return
	}
	ctx.Flash.Success(ctx.Tr("actions.variables.deletion.success"))
	ctx.JSONRedirect(vCtx.RedirectLink)
}
