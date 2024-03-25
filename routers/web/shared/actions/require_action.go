// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// WIP RequireAction

package actions

import (
	"errors"

	actions_model "code.gitea.io/gitea/models/actions"
	//repo_model    "code.gitea.io/gitea/models/repo"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
	action_service "code.gitea.io/gitea/services/actions"

)

// RequireActionsList prepares data for workflow actions list
func RequireActionsList(ctx *context.Context, opts actions_model.FindRequireActionOptions) {
	//all_opts := repo_model.FindEnabledGlobalWorkflowsOptions {}
	//avalible_workflows, count, err := db.FindAndCount[repo_model.RepoUnit](ctx, all_opts)
	require_actions, count, err := db.FindAndCount[actions_model.RequireAction](ctx, opts)

	if err != nil {
		ctx.ServerError("CountRequireAction", err)
		return
	}

	if err := actions_model.RequireActionList(require_actions).LoadAttributes(ctx); err != nil {
		ctx.ServerError("RequireActionLoadAttributes", err)
		return
	}

	//ctx.Data["Link"] = opts.Link
	ctx.Data["RequireActions"] = require_actions
	ctx.Data["Total"] = count
	ctx.Data["RequireActionsOrgID"] = opts.OrgID
	//ctx.Data["Sort"] = opts.Sort

	pager := context.NewPagination(int(count), opts.PageSize, opts.Page, 5)

	ctx.Data["Page"] = pager
	workflow_list, err := actions_model.ListAvailableWorkflows(ctx, opts.OrgID)
	if err != nil {
		ctx.ServerError("ListAvailableWorkflows", err)
		return
	}
	/*
		var global_workflow []string
		for _, workflow := range workflow_list {
			global_workflow = append(global_workflow, workflow.Data)
		}
		ctx.Data["ListAvailableWorkflows"] = global_workflow
	*/
	ctx.Data["ListAvailableWorkflows"] = workflow_list
}

// RequireActionsDetails prepares data for RequireActions edit page
func RequireActionsDetails(ctx *context.Context, page int, actionID, ownerID, repoID int64, link string) {
	require_action, err := actions_model.GetRequireActionByID(ctx, actionID)
	if err != nil {
		ctx.ServerError("GetRequireActionByID", err)
		return
	}
	if err := require_action.LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}
	if !require_action.Editable(ownerID, repoID) {
		err = errors.New("no permission to edit this require action")
		ctx.NotFound("RequireActionDetails", err)
		return
	}

	ctx.Data["RequireAction"] = require_action

	opts := actions_model.FindRequireActionOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: 30,
		},
	}

	require_action_list, count, err := db.FindAndCount[actions_model.RequireAction](ctx, opts)
	if err != nil {
		ctx.ServerError("CountRequireActions", err)
		return
	}

	if err = actions_model.RequireActionList(require_action_list).LoadAttributes(ctx); err != nil {
		ctx.ServerError("RequireActionListLoadAttributes", err)
		return
	}

	ctx.Data["RequireActionList"] = require_action_list
	pager := context.NewPagination(int(count), opts.PageSize, opts.Page, 5)
	ctx.Data["Page"] = pager
}

// RequireActionEditPost response for edit require action details
func RequireActionEditPost(ctx *context.Context, actionID, ownerID, repoID int64, redirectTo string) {
	require_action, err := actions_model.GetRequireActionByID(ctx, actionID)
	if err != nil {
		log.Warn("RequireActionEditPost.GetRequireActionByID failed: %v, link: %s", err, ctx.Req.URL)
		ctx.ServerError("RequireActionEditPost.GetRequireActionByID", err)
		return
	}
	if !require_action.Editable(ownerID, repoID) {
		ctx.NotFound("RequireAction.Editable", util.NewPermissionDeniedErrorf("no permission to edit this require action"))
		return
	}

	form := web.GetForm(ctx).(*forms.EditRequireActionForm)
	require_action.Link = form.Link

	_, err = actions_model.UpdateRequireAction(ctx, require_action)
	if err != nil {
		log.Warn("RequireActionEditPost.UpdateRequireAction failed: %v, link: %s", err, ctx.Req.URL)
		ctx.Flash.Warning(ctx.Tr("actions.runners.update_require_action_failed"))
		ctx.Redirect(redirectTo)
		return
	}

	log.Debug("RequireActionEditPost success: %s", ctx.Req.URL)

	ctx.Flash.Success(ctx.Tr("actions.runners.update_require_action_success"))
	ctx.Redirect(redirectTo)
}

// RequireActionUpdateLink reset workflow link
/*func RequireActionUpdateLink(ctx *context.Context, ownerID, repoID int64, link string, redirectTo string) {
	_, err := actions_model.RequireActionUpdateLink(ctx, ownerID, repoID, link)
	if err != nil {
		ctx.ServerError("RequireActionUpdateLink", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("actions.runners.update_require_action_link_success"))
	ctx.Redirect(redirectTo)
}
*/
// RequireActionDeletePost response for deleting a require action workflow
/* func RequireActionDeletePost(ctx *context.Context, actionID int64,
	successRedirectTo, failedRedirectTo string,
) {
	if err := actions_model.DeleteRequireAction(ctx, actionID); err != nil {
		log.Warn("RequireActionDeletePost.DeleteRequireAction failed: %v, link: %s", err, ctx.Req.Link)
		ctx.Flash.Warning(ctx.Tr("actions.runners.delete_require_action_failed"))

		ctx.JSONRedirect(failedRedirectTo)
		return
	}

	log.Info("RequireActionDeletePost success: %s", ctx.Req.Link)

	ctx.Flash.Success(ctx.Tr("actions.runners.delete_require_action_success"))

	ctx.JSONRedirect(successRedirectTo)
}
*/

// CreateRequireAction create require action workflow
func CreateRequireAction(ctx *context.Context, orgID int64, redirectURL string) {
	form := web.GetForm(ctx).(*forms.EditRequireActionForm)

	v, err := action_service.CreateRequireAction(ctx, orgID, form.RepoID, ReserveLineBreakForTextarea(form.Data), form.Link)

	if err != nil {
		log.Error("CreateRequireAction error: %v", err)
		ctx.JSONError(ctx.Tr("actions.require_action.creation.failed"))
		return
	}
	ctx.Flash.Success(ctx.Tr("actions.require_action.creation.success", v.Data))
	ctx.JSONRedirect(redirectURL)
}

// UpdateRequireAction update require action workflow
func UpdateRequireAction(ctx *context.Context, redirectURL string) {

}

// DeleteRequireAction update require action workflow
func DeleteRequireAction(ctx *context.Context, redirectURL string) {
	id := ctx.FormInt64("require_action_id")

	err := action_service.DeleteRequireActionByID(ctx, id)
	if	err != nil {
		log.Error("DeDeleteRequireAction [%d] failed: %v", id, err)
		ctx.JSONError(ctx.Tr("actions.require_action.deletion.failed"))
		return
	}
	ctx.Flash.Success(ctx.Tr("actions.require_action.deletion.success"))
	ctx.JSONRedirect(redirectURL)
}

// SetRequireActionDeletePost response for deleting a require action workflow
func SetRequireActionsContext(ctx *context.Context) {

}
