// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/web"
	actions_service "code.gitea.io/gitea/services/actions"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
)

// SetRequireActionDeletePost response for deleting a require action workflow
func SetRequireActionContext(ctx *context.Context, opts actions_model.FindRequireActionOptions) {
	requireActions, count, err := db.FindAndCount[actions_model.RequireAction](ctx, opts)
	if err != nil {
		ctx.ServerError("CountRequireActions", err)
		return
	}
	ctx.Data["RequireActions"] = requireActions
	ctx.Data["Total"] = count
	ctx.Data["OrgID"] = ctx.Org.Organization.ID
	ctx.Data["OrgName"] = ctx.Org.Organization.Name
	pager := context.NewPagination(int(count), opts.PageSize, opts.Page, 5)
	ctx.Data["Page"] = pager
}

// get all the available enable global workflow in the org's repo
func GlobalEnableWorkflow(ctx *context.Context, orgID int64) {
	var gwfList []actions_model.GlobalWorkflow
	orgRepos, err := org_model.GetOrgRepositories(ctx, orgID)
	if err != nil {
		ctx.ServerError("GlobalEnableWorkflows get org repos: ", err)
		return
	}
	for _, repo := range orgRepos {
		err := repo.LoadUnits(ctx)
		if err != nil {
			ctx.ServerError("GlobalEnableWorkflows LoadUnits : ", err)
		}
		actionsConfig := repo.MustGetUnit(ctx, unit.TypeActions).ActionsConfig()
		enabledWorkflows := actionsConfig.GetGlobalWorkflow()
		for _, workflow := range enabledWorkflows {
			gwf := actions_model.GlobalWorkflow{
				RepoName: repo.Name,
				Filename: workflow,
			}
			gwfList = append(gwfList, gwf)
		}
	}
	ctx.Data["GlobalEnableWorkflows"] = gwfList
}

func CreateRequireAction(ctx *context.Context, orgID int64, redirectURL string) {
	ctx.Data["OrgID"] = ctx.Org.Organization.ID
	form := web.GetForm(ctx).(*forms.RequireActionForm)
	v, err := actions_service.CreateRequireAction(ctx, orgID, form.RepoName, form.WorkflowName)
	if err != nil {
		log.Error("CreateRequireAction: %v", err)
		ctx.JSONError(ctx.Tr("actions.require_action.creation.failed"))
		return
	}
	ctx.Flash.Success(ctx.Tr("actions.require_action.creation.success", v.WorkflowName))
	ctx.JSONRedirect(redirectURL)
}

func DeleteRequireAction(ctx *context.Context, redirectURL string) {
	id := ctx.PathParamInt64(":require_action_id")

	if err := actions_service.DeleteRequireActionByID(ctx, id); err != nil {
		log.Error("Delete RequireAction [%d] failed: %v", id, err)
		ctx.JSONError(ctx.Tr("actions.require_action.deletion.failed"))
		return
	}
	ctx.Flash.Success(ctx.Tr("actions.require_action.deletion.success"))
	ctx.JSONRedirect(redirectURL)
}
