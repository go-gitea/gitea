// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"net/http"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	actions_service "code.gitea.io/gitea/services/actions"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
)

const (
	tplRepoVariables  base.TplName = "repo/settings/actions"
	tplOrgVariables   base.TplName = "org/settings/actions"
	tplUserVariables  base.TplName = "user/settings/actions"
	tplAdminVariables base.TplName = "admin/actions"
)

type variablesCtx struct {
	OwnerID           int64
	RepoID            int64
	IsRepo            bool
	IsOrg             bool
	IsUser            bool
	IsGlobal          bool
	VariablesTemplate base.TplName
	RedirectLink      string
}

func getVariablesCtx(ctx *context.Context) (*variablesCtx, error) {
	if ctx.Data["PageIsRepoSettings"] == true {
		return &variablesCtx{
			OwnerID:           0,
			RepoID:            ctx.Repo.Repository.ID,
			IsRepo:            true,
			VariablesTemplate: tplRepoVariables,
			RedirectLink:      ctx.Repo.RepoLink + "/settings/actions/variables",
		}, nil
	}

	if ctx.Data["PageIsOrgSettings"] == true {
		err := shared_user.LoadHeaderCount(ctx)
		if err != nil {
			ctx.ServerError("LoadHeaderCount", err)
			return nil, nil
		}
		return &variablesCtx{
			OwnerID:           ctx.ContextUser.ID,
			RepoID:            0,
			IsOrg:             true,
			VariablesTemplate: tplOrgVariables,
			RedirectLink:      ctx.Org.OrgLink + "/settings/actions/variables",
		}, nil
	}

	if ctx.Data["PageIsUserSettings"] == true {
		return &variablesCtx{
			OwnerID:           ctx.Doer.ID,
			RepoID:            0,
			IsUser:            true,
			VariablesTemplate: tplUserVariables,
			RedirectLink:      setting.AppSubURL + "/user/settings/actions/variables",
		}, nil
	}

	if ctx.Data["PageIsAdmin"] == true {
		return &variablesCtx{
			OwnerID:           0,
			RepoID:            0,
			IsGlobal:          true,
			VariablesTemplate: tplAdminVariables,
			RedirectLink:      setting.AppSubURL + "/-/admin/actions/variables",
		}, nil
	}

	return nil, errors.New("unable to set Variables context")
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

	form := web.GetForm(ctx).(*forms.EditVariableForm)

	v, err := actions_service.CreateVariable(ctx, vCtx.OwnerID, vCtx.RepoID, form.Name, form.Data)
	if err != nil {
		log.Error("CreateVariable: %v", err)
		ctx.JSONError(ctx.Tr("actions.variables.creation.failed"))
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

	form := web.GetForm(ctx).(*forms.EditVariableForm)
	variable.Name = form.Name
	variable.Data = form.Data

	if ok, err := actions_service.UpdateVariableNameData(ctx, variable); err != nil || !ok {
		log.Error("UpdateVariable: %v", err)
		ctx.JSONError(ctx.Tr("actions.variables.update.failed"))
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
	case vCtx.IsGlobal:
		// do nothing
	default:
		panic("invalid actions variable")
	}

	got, err := actions_model.FindVariables(ctx, opts)
	if err != nil {
		ctx.ServerError("FindVariables", err)
		return nil
	} else if len(got) == 0 {
		ctx.NotFound("FindVariables", nil)
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
