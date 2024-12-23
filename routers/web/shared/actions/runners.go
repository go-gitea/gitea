// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
)

// RunnersList prepares data for runners list
func RunnersList(ctx *context.Context, opts actions_model.FindRunnerOptions) {
	runners, count, err := db.FindAndCount[actions_model.ActionRunner](ctx, opts)
	if err != nil {
		ctx.ServerError("CountRunners", err)
		return
	}

	if err := actions_model.RunnerList(runners).LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}

	// ownid=0,repo_id=0,means this token is used for global
	var token *actions_model.ActionRunnerToken
	token, err = actions_model.GetLatestRunnerToken(ctx, opts.OwnerID, opts.RepoID)
	if errors.Is(err, util.ErrNotExist) || (token != nil && !token.IsActive) {
		token, err = actions_model.NewRunnerToken(ctx, opts.OwnerID, opts.RepoID)
		if err != nil {
			ctx.ServerError("CreateRunnerToken", err)
			return
		}
	} else if err != nil {
		ctx.ServerError("GetLatestRunnerToken", err)
		return
	}

	ctx.Data["Keyword"] = opts.Filter
	ctx.Data["Runners"] = runners
	ctx.Data["Total"] = count
	ctx.Data["RegistrationToken"] = token.Token
	ctx.Data["RunnerOwnerID"] = opts.OwnerID
	ctx.Data["RunnerRepoID"] = opts.RepoID
	ctx.Data["SortType"] = opts.Sort

	pager := context.NewPagination(int(count), opts.PageSize, opts.Page, 5)

	ctx.Data["Page"] = pager
}

// RunnerDetails prepares data for runners edit page
func RunnerDetails(ctx *context.Context, page int, runnerID, ownerID, repoID int64) {
	runner, err := actions_model.GetRunnerByID(ctx, runnerID)
	if err != nil {
		ctx.ServerError("GetRunnerByID", err)
		return
	}
	if err := runner.LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}
	if !runner.Editable(ownerID, repoID) {
		err = errors.New("no permission to edit this runner")
		ctx.NotFound("RunnerDetails", err)
		return
	}

	ctx.Data["Runner"] = runner

	opts := actions_model.FindTaskOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: 30,
		},
		Status:   actions_model.StatusUnknown, // Unknown means all
		RunnerID: runner.ID,
	}

	tasks, count, err := db.FindAndCount[actions_model.ActionTask](ctx, opts)
	if err != nil {
		ctx.ServerError("CountTasks", err)
		return
	}

	if err = actions_model.TaskList(tasks).LoadAttributes(ctx); err != nil {
		ctx.ServerError("TasksLoadAttributes", err)
		return
	}

	ctx.Data["Tasks"] = tasks
	pager := context.NewPagination(int(count), opts.PageSize, opts.Page, 5)
	ctx.Data["Page"] = pager
}

// RunnerDetailsEditPost response for edit runner details
func RunnerDetailsEditPost(ctx *context.Context, runnerID, ownerID, repoID int64, redirectTo string) {
	runner, err := actions_model.GetRunnerByID(ctx, runnerID)
	if err != nil {
		log.Warn("RunnerDetailsEditPost.GetRunnerByID failed: %v, url: %s", err, ctx.Req.URL)
		ctx.ServerError("RunnerDetailsEditPost.GetRunnerByID", err)
		return
	}
	if !runner.Editable(ownerID, repoID) {
		ctx.NotFound("RunnerDetailsEditPost.Editable", util.NewPermissionDeniedErrorf("no permission to edit this runner"))
		return
	}

	form := web.GetForm(ctx).(*forms.EditRunnerForm)
	runner.Description = form.Description

	err = actions_model.UpdateRunner(ctx, runner, "description")
	if err != nil {
		log.Warn("RunnerDetailsEditPost.UpdateRunner failed: %v, url: %s", err, ctx.Req.URL)
		ctx.Flash.Warning(ctx.Tr("actions.runners.update_runner_failed"))
		ctx.Redirect(redirectTo)
		return
	}

	log.Debug("RunnerDetailsEditPost success: %s", ctx.Req.URL)

	ctx.Flash.Success(ctx.Tr("actions.runners.update_runner_success"))
	ctx.Redirect(redirectTo)
}

// RunnerResetRegistrationToken reset registration token
func RunnerResetRegistrationToken(ctx *context.Context, ownerID, repoID int64, redirectTo string) {
	_, err := actions_model.NewRunnerToken(ctx, ownerID, repoID)
	if err != nil {
		ctx.ServerError("ResetRunnerRegistrationToken", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("actions.runners.reset_registration_token_success"))
	ctx.JSONRedirect(redirectTo)
}

// RunnerDeletePost response for deleting a runner
func RunnerDeletePost(ctx *context.Context, runnerID int64,
	successRedirectTo, failedRedirectTo string,
) {
	if err := actions_model.DeleteRunner(ctx, runnerID); err != nil {
		log.Warn("DeleteRunnerPost.UpdateRunner failed: %v, url: %s", err, ctx.Req.URL)
		ctx.Flash.Warning(ctx.Tr("actions.runners.delete_runner_failed"))

		ctx.JSONRedirect(failedRedirectTo)
		return
	}

	log.Info("DeleteRunnerPost success: %s", ctx.Req.URL)

	ctx.Flash.Success(ctx.Tr("actions.runners.delete_runner_success"))

	ctx.JSONRedirect(successRedirectTo)
}
