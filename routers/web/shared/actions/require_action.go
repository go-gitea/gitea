// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// WIP RequireAction

package actions

import (

    actions_model   "code.gitea.io/gitea/models/actions"
    org_model       "code.gitea.io/gitea/models/organization"
    "code.gitea.io/gitea/models/db"
    "code.gitea.io/gitea/modules/log"
    //"code.gitea.io/gitea/modules/util"
    "code.gitea.io/gitea/models/unit"
    "code.gitea.io/gitea/modules/web"
    "code.gitea.io/gitea/services/forms"
    actions_service "code.gitea.io/gitea/services/actions"

    "code.gitea.io/gitea/services/context"
)

// SetRequireActionDeletePost response for deleting a require action workflow
func SetRequireActionContext(ctx *context.Context, opts actions_model.FindRequireActionOptions) {
    require_actions, count, err := db.FindAndCount[actions_model.RequireAction](ctx, opts)
    if err != nil {
        ctx.ServerError("CountRequireActions", err)
        return
    }
    ctx.Data["RequireActions"] = require_actions
    ctx.Data["Total"] = count
    ctx.Data["OrgID"] = ctx.Org.Organization.ID
    ctx.Data["OrgName"] = ctx.Org.Organization.Name
    pager := context.NewPagination(int(count), opts.PageSize, opts.Page, 5)
    ctx.Data["Page"] = pager
}

// get all the available enable global workflow in the org's repo
func GlobalEnableWorkflow(ctx *context.Context, orgID int64){
    var gwfList []actions_model.GlobalWorkflow
    orgRepos, err := org_model.GetOrgRepositories(ctx, orgID)
    if err != nil {
        ctx.ServerError("GlobalEnableWorkflows get org repos: ", err)
        return
    }
    for _, repo := range orgRepos {
        repo.LoadUnits(ctx)
        actionsConfig := repo.MustGetUnit(ctx, unit.TypeActions).ActionsConfig()
        enabledWorkflows := actionsConfig.GetGlobalWorkflow()
        for _, workflow := range enabledWorkflows {
            gwf := actions_model.GlobalWorkflow{
                RepoName:        repo.Name,
                Filename:    workflow,
            }
            gwfList = append(gwfList, gwf)
        }
    }
    ctx.Data["GlobalEnableWorkflows"] = gwfList
}

func CreateRequireAction(ctx *context.Context, orgID int64, redirectURL string){
    ctx.Data["OrgID"] = ctx.Org.Organization.ID
    form := web.GetForm(ctx).(*forms.RequireActionForm)
    // log.Error("org %d, repo_name: %s, workflow_name %s", orgID, form.RepoName, form.WorkflowName)
    log.Error("org %d, repo_name: %+v", orgID, form)
    v, err := actions_service.CreateRequireAction(ctx, orgID, form.RepoName, form.WorkflowName)
    if err != nil {
        log.Error("CreateRequireAction: %v", err)
        ctx.JSONError(ctx.Tr("actions.require_action.creation.failed"))
        return
    }
    ctx.Flash.Success(ctx.Tr("actions.require_action.creation.success", v.WorkflowName))
    ctx.JSONRedirect(redirectURL)
}
