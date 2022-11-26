package common

import (
	"errors"
	"net/http"
	"strings"

	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
)

// RunnersList render common runners list page
func RunnersList(ctx *context.Context, tplName base.TplName, opts bots_model.FindRunnerOptions) {

	count, err := bots_model.CountRunners(opts)
	if err != nil {
		ctx.ServerError("AdminRunners", err)
		return
	}

	runners, err := bots_model.FindRunners(opts)
	if err != nil {
		ctx.ServerError("AdminRunners", err)
		return
	}
	if err := runners.LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}

	// ownid=0,repo_id=0,means this token is used for global
	var token *bots_model.RunnerToken
	token, err = bots_model.GetUnactivatedRunnerToken(opts.OwnerID, opts.RepoID)
	if _, ok := err.(bots_model.ErrRunnerTokenNotExist); ok {
		token, err = bots_model.NewRunnerToken(opts.OwnerID, opts.RepoID)
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
	ctx.Data["RunnerOnwerID"] = opts.OwnerID
	ctx.Data["RunnerRepoID"] = opts.RepoID

	pager := context.NewPagination(int(count), opts.PageSize, opts.Page, 5)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplName)
}

// RunnerDetails render runner details page
func RunnerDetails(ctx *context.Context, tplName base.TplName, page int, runnerID int64, ownerID int64, repoID int64) {
	runner, err := bots_model.GetRunnerByID(runnerID)
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

	opts := bots_model.FindTaskOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: 30,
		},
		Status:      bots_model.StatusUnknown, // Unknown means all
		IDOrderDesc: true,
		RunnerID:    runner.ID,
	}

	count, err := bots_model.CountTasks(ctx, opts)
	if err != nil {
		ctx.ServerError("CountTasks", err)
		return
	}

	tasks, _, err := bots_model.FindTasks(ctx, opts)
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

	ctx.HTML(http.StatusOK, tplName)
}

// RunnerDetailsEditPost response for edit runner details
func RunnerDetailsEditPost(ctx *context.Context, runnerID int64, ownerID int64, repoID int64, redirectTo string) {
	runner, err := bots_model.GetRunnerByID(runnerID)
	if err != nil {
		log.Warn("RunnerDetailsEditPost.GetRunnerByID failed: %v, url: %s", err, ctx.Req.URL)
		ctx.ServerError("RunnerDetailsEditPost.GetRunnerByID", err)
		return
	}
	if !runner.Editable(ownerID, repoID) {
		err = errors.New("no permission to edit this runner")
		ctx.NotFound("RunnerDetailsEditPost.Editable", err)
		return
	}

	form := web.GetForm(ctx).(*forms.EditRunnerForm)
	runner.Description = form.Description
	runner.CustomLabels = strings.Split(form.CustomLabels, ",")

	err = bots_model.UpdateRunner(ctx, runner, "description", "custom_labels")
	if err != nil {
		log.Warn("RunnerDetailsEditPost.UpdateRunner failed: %v, url: %s", err, ctx.Req.URL)
		ctx.Flash.Warning(ctx.Tr("admin.runners.update_runner_failed"))
		ctx.Redirect(redirectTo)
		return
	}

	log.Debug("RunnerDetailsEditPost success: %s", ctx.Req.URL)

	ctx.Flash.Success(ctx.Tr("admin.runners.update_runner_success"))
	ctx.Redirect(redirectTo)
}

// RunnerResetRegistrationToken reset registration token
func RunnerResetRegistrationToken(ctx *context.Context, ownerID, repoID int64, redirectTo string) {
	_, err := bots_model.NewRunnerToken(ownerID, repoID)
	if err != nil {
		ctx.ServerError("ResetRunnerRegistrationToken", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("admin.runners.reset_registration_token_success"))
	ctx.Redirect(redirectTo)
}

// RunnerDeletePost response for deleting a runner
func RunnerDeletePost(ctx *context.Context, runnerID int64,
	successRedirectTo, failedRedirectTo string) {
	runner, err := bots_model.GetRunnerByID(runnerID)
	if err != nil {
		log.Warn("DeleteRunnerPost.GetRunnerByID failed: %v, url: %s", err, ctx.Req.URL)
		ctx.ServerError("DeleteRunnerPost.GetRunnerByID", err)
		return
	}

	err = bots_model.DeleteRunner(ctx, runner)
	if err != nil {
		log.Warn("DeleteRunnerPost.UpdateRunner failed: %v, url: %s", err, ctx.Req.URL)
		ctx.Flash.Warning(ctx.Tr("runners.delete_runner_failed"))
		ctx.Redirect(failedRedirectTo)
		return
	}

	log.Info("DeleteRunnerPost success: %s", ctx.Req.URL)

	ctx.Flash.Success(ctx.Tr("runners.delete_runner_success"))
	ctx.Redirect(successRedirectTo)
}
