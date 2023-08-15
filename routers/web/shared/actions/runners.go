// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"net/http"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
)

// RunnersList prepares data for runners list
func RunnersList(ctx *context.Context, opts actions_model.FindRunnerOptions) {
	count, err := actions_model.CountRunners(ctx, opts)
	if err != nil {
		ctx.ServerError("CountRunners", err)
		return
	}

	runners, err := actions_model.FindRunners(ctx, opts)
	if err != nil {
		ctx.ServerError("FindRunners", err)
		return
	}
	if err := runners.LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}

	// ownid=0,repo_id=0,means this token is used for global
	var token *actions_model.ActionRunnerToken
	token, err = actions_model.GetUnactivatedRunnerToken(ctx, opts.OwnerID, opts.RepoID)
	if errors.Is(err, util.ErrNotExist) {
		token, err = actions_model.NewRunnerToken(ctx, opts.OwnerID, opts.RepoID)
		if err != nil {
			ctx.ServerError("CreateRunnerToken", err)
			return
		}
	} else if err != nil {
		ctx.ServerError("GetUnactivatedRunnerToken", err)
		return
	}

	ctx.Data["Keyword"] = opts.Filter
	ctx.Data["Runners"] = runners
	ctx.Data["Total"] = count
	ctx.Data["RegistrationToken"] = token.Token
	ctx.Data["RunnerOwnerID"] = opts.OwnerID
	ctx.Data["RunnerRepoID"] = opts.RepoID

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
		Status:      actions_model.StatusUnknown, // Unknown means all
		IDOrderDesc: true,
		RunnerID:    runner.ID,
	}

	count, err := actions_model.CountTasks(ctx, opts)
	if err != nil {
		ctx.ServerError("CountTasks", err)
		return
	}

	tasks, err := actions_model.FindTasks(ctx, opts)
	if err != nil {
		ctx.ServerError("FindTasks", err)
		return
	}
	if err = tasks.LoadAttributes(ctx); err != nil {
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
	runner.CustomLabels = splitLabels(form.CustomLabels)

	err = actions_model.UpdateRunner(ctx, runner, "description", "custom_labels")
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
	ctx.Redirect(redirectTo)
}

// RunnerDeletePost response for deleting a runner
func RunnerDeletePost(ctx *context.Context, runnerID int64,
	successRedirectTo, failedRedirectTo string,
) {
	if err := actions_model.DeleteRunner(ctx, runnerID); err != nil {
		log.Warn("DeleteRunnerPost.UpdateRunner failed: %v, url: %s", err, ctx.Req.URL)
		ctx.Flash.Warning(ctx.Tr("actions.runners.delete_runner_failed"))

		ctx.JSON(http.StatusOK, map[string]any{
			"redirect": failedRedirectTo,
		})
		return
	}

	log.Info("DeleteRunnerPost success: %s", ctx.Req.URL)

	ctx.Flash.Success(ctx.Tr("actions.runners.delete_runner_success"))

	ctx.JSON(http.StatusOK, map[string]any{
		"redirect": successRedirectTo,
	})
}

func splitLabels(s string) []string {
	labels := strings.Split(s, ",")
	for i, v := range labels {
		labels[i] = strings.TrimSpace(v)
	}
	return labels
}
