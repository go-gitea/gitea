// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"net/http"
	"net/url"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
)

const (
	// TODO: Separate secrets from runners when layout is ready
	tplRepoRunners     base.TplName = "repo/settings/actions"
	tplOrgRunners      base.TplName = "org/settings/actions"
	tplAdminRunners    base.TplName = "admin/actions"
	tplUserRunners     base.TplName = "user/settings/actions"
	tplRepoRunnerEdit  base.TplName = "repo/settings/runner_edit"
	tplOrgRunnerEdit   base.TplName = "org/settings/runners_edit"
	tplAdminRunnerEdit base.TplName = "admin/runners/edit"
	tplUserRunnerEdit  base.TplName = "user/settings/runner_edit"
)

type runnersCtx struct {
	OwnerID            int64
	RepoID             int64
	IsRepo             bool
	IsOrg              bool
	IsAdmin            bool
	IsUser             bool
	RunnersTemplate    base.TplName
	RunnerEditTemplate base.TplName
	RedirectLink       string
}

func getRunnersCtx(ctx *context.Context) (*runnersCtx, error) {
	if ctx.Data["PageIsRepoSettings"] == true {
		return &runnersCtx{
			RepoID:             ctx.Repo.Repository.ID,
			OwnerID:            0,
			IsRepo:             true,
			RunnersTemplate:    tplRepoRunners,
			RunnerEditTemplate: tplRepoRunnerEdit,
			RedirectLink:       ctx.Repo.RepoLink + "/settings/actions/runners/",
		}, nil
	}

	if ctx.Data["PageIsOrgSettings"] == true {
		err := shared_user.LoadHeaderCount(ctx)
		if err != nil {
			ctx.ServerError("LoadHeaderCount", err)
			return nil, nil
		}
		return &runnersCtx{
			RepoID:             0,
			OwnerID:            ctx.Org.Organization.ID,
			IsOrg:              true,
			RunnersTemplate:    tplOrgRunners,
			RunnerEditTemplate: tplOrgRunnerEdit,
			RedirectLink:       ctx.Org.OrgLink + "/settings/actions/runners/",
		}, nil
	}

	if ctx.Data["PageIsAdmin"] == true {
		return &runnersCtx{
			RepoID:             0,
			OwnerID:            0,
			IsAdmin:            true,
			RunnersTemplate:    tplAdminRunners,
			RunnerEditTemplate: tplAdminRunnerEdit,
			RedirectLink:       setting.AppSubURL + "/-/admin/actions/runners/",
		}, nil
	}

	if ctx.Data["PageIsUserSettings"] == true {
		return &runnersCtx{
			OwnerID:            ctx.Doer.ID,
			RepoID:             0,
			IsUser:             true,
			RunnersTemplate:    tplUserRunners,
			RunnerEditTemplate: tplUserRunnerEdit,
			RedirectLink:       setting.AppSubURL + "/user/settings/actions/runners/",
		}, nil
	}

	return nil, errors.New("unable to set Runners context")
}

// Runners render settings/actions/runners page for repo level
func Runners(ctx *context.Context) {
	ctx.Data["PageIsSharedSettingsRunners"] = true
	ctx.Data["Title"] = ctx.Tr("actions.actions")
	ctx.Data["PageType"] = "runners"

	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerError("getRunnersCtx", err)
		return
	}

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	opts := actions_model.FindRunnerOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: 100,
		},
		Sort:   ctx.Req.URL.Query().Get("sort"),
		Filter: ctx.Req.URL.Query().Get("q"),
	}
	if rCtx.IsRepo {
		opts.RepoID = rCtx.RepoID
		opts.WithAvailable = true
	} else if rCtx.IsOrg || rCtx.IsUser {
		opts.OwnerID = rCtx.OwnerID
		opts.WithAvailable = true
	}

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

	ctx.HTML(http.StatusOK, rCtx.RunnersTemplate)
}

// RunnersEdit renders runner edit page for repository level
func RunnersEdit(ctx *context.Context) {
	ctx.Data["PageIsSharedSettingsRunners"] = true
	ctx.Data["Title"] = ctx.Tr("actions.runners.edit_runner")
	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerError("getRunnersCtx", err)
		return
	}

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	runnerID := ctx.PathParamInt64("runnerid")
	ownerID := rCtx.OwnerID
	repoID := rCtx.RepoID

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

	ctx.HTML(http.StatusOK, rCtx.RunnerEditTemplate)
}

func RunnersEditPost(ctx *context.Context) {
	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerError("getRunnersCtx", err)
		return
	}

	runnerID := ctx.PathParamInt64("runnerid")
	ownerID := rCtx.OwnerID
	repoID := rCtx.RepoID
	redirectTo := rCtx.RedirectLink

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

func ResetRunnerRegistrationToken(ctx *context.Context) {
	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerError("getRunnersCtx", err)
		return
	}

	ownerID := rCtx.OwnerID
	repoID := rCtx.RepoID
	redirectTo := rCtx.RedirectLink

	if _, err := actions_model.NewRunnerToken(ctx, ownerID, repoID); err != nil {
		ctx.ServerError("ResetRunnerRegistrationToken", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("actions.runners.reset_registration_token_success"))
	ctx.JSONRedirect(redirectTo)
}

// RunnerDeletePost response for deleting runner
func RunnerDeletePost(ctx *context.Context) {
	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerError("getRunnersCtx", err)
		return
	}

	runner := findActionsRunner(ctx, rCtx)
	if ctx.Written() {
		return
	}

	if !runner.Editable(rCtx.OwnerID, rCtx.RepoID) {
		ctx.NotFound("RunnerDeletePost", util.NewPermissionDeniedErrorf("no permission to delete this runner"))
		return
	}

	successRedirectTo := rCtx.RedirectLink
	failedRedirectTo := rCtx.RedirectLink + url.PathEscape(ctx.PathParam("runnerid"))

	if err := actions_model.DeleteRunner(ctx, runner.ID); err != nil {
		log.Warn("DeleteRunnerPost.UpdateRunner failed: %v, url: %s", err, ctx.Req.URL)
		ctx.Flash.Warning(ctx.Tr("actions.runners.delete_runner_failed"))

		ctx.JSONRedirect(failedRedirectTo)
		return
	}

	log.Info("DeleteRunnerPost success: %s", ctx.Req.URL)

	ctx.Flash.Success(ctx.Tr("actions.runners.delete_runner_success"))

	ctx.JSONRedirect(successRedirectTo)
}

func RedirectToDefaultSetting(ctx *context.Context) {
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/actions/runners")
}

func findActionsRunner(ctx *context.Context, rCtx *runnersCtx) *actions_model.ActionRunner {
	runnerID := ctx.PathParamInt64("runnerid")
	opts := &actions_model.FindRunnerOptions{
		IDs: []int64{runnerID},
	}
	switch {
	case rCtx.IsRepo:
		opts.RepoID = rCtx.RepoID
		if opts.RepoID == 0 {
			panic("repoID is 0")
		}
	case rCtx.IsOrg, rCtx.IsUser:
		opts.OwnerID = rCtx.OwnerID
		if opts.OwnerID == 0 {
			panic("ownerID is 0")
		}
	case rCtx.IsAdmin:
		// do nothing
	default:
		panic("invalid actions runner context")
	}

	got, err := db.Find[actions_model.ActionRunner](ctx, opts)
	if err != nil {
		ctx.ServerError("FindRunner", err)
		return nil
	} else if len(got) == 0 {
		ctx.NotFound("FindRunner", errors.New("runner not found"))
		return nil
	}

	return got[0]
}
