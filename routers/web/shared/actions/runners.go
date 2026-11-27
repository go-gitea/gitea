// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	stdctx "context"
	"net/http"
	"net/url"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/modules/log"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/web"
	"gitea.dev/services/audit"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
)

var runnerEditTemplates = map[settingsScopeKind]templates.TplName{
	settingsScopeRepo:  "repo/settings/runner_edit",
	settingsScopeOrg:   "org/settings/runners_edit",
	settingsScopeUser:  "user/settings/runner_edit",
	settingsScopeAdmin: "admin/runners/edit",
}

type runnersCtx struct {
	*settingsScope
	RedirectLink string
}

func getRunnersCtx(ctx *context.Context) (*runnersCtx, error) {
	scope, err := getSettingsScope(ctx)
	if err != nil {
		return nil, err
	}
	return &runnersCtx{settingsScope: scope, RedirectLink: scope.LinkPrefix + "/runners/"}, nil
}

// Runners render settings/actions/runners page for repo level
func Runners(ctx *context.Context) {
	ctx.Data["PageIsSharedSettingsRunners"] = true
	ctx.Data["Title"] = ctx.Tr("actions.runners")
	ctx.Data["PageType"] = "runners"

	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	opts := actions_model.FindRunnerOptions{
		ListOptions: db.ListOptions{
			Page:     max(ctx.FormInt("page"), 1),
			PageSize: 100,
		},
		Sort:          ctx.Req.URL.Query().Get("sort"),
		Filter:        ctx.Req.URL.Query().Get("q"),
		OwnerID:       rCtx.OwnerID,
		RepoID:        rCtx.RepoID,
		WithAvailable: !rCtx.IsAdmin(),
	}

	runners, count, err := db.FindAndCount[actions_model.ActionRunner](ctx, opts)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	if err := actions_model.RunnerList(runners).LoadAttributes(ctx); err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	// ownid=0,repo_id=0,means this token is used for global
	token, err := actions_model.GetOrCreateActiveRunnerToken(ctx, opts.OwnerID, opts.RepoID)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	ctx.Data["Keyword"] = opts.Filter
	ctx.Data["Runners"] = runners
	ctx.Data["Total"] = count
	ctx.Data["RegistrationToken"] = token.Token
	ctx.Data["RunnerOwnerID"] = opts.OwnerID
	ctx.Data["RunnerRepoID"] = opts.RepoID
	ctx.Data["SortType"] = opts.Sort
	ctx.Data["AllowBulkActions"] = rCtx.IsAdmin()

	pager := context.NewPagerBuilder(ctx).TotalCount(count).PerPageLimit(opts.PageSize).CurPage(opts.Page).Build()

	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, rCtx.Template)
}

// RunnersEdit renders runner edit page for repository level
func RunnersEdit(ctx *context.Context) {
	ctx.Data["PageIsSharedSettingsRunners"] = true
	ctx.Data["Title"] = ctx.Tr("actions.runners.edit_runner")
	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	page := max(ctx.FormInt("page"), 1)

	runnerID := ctx.PathParamInt64("runnerid")
	ownerID := rCtx.OwnerID
	repoID := rCtx.RepoID

	runner, err := actions_model.GetRunnerByID(ctx, runnerID)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	if err := runner.LoadAttributes(ctx); err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	if !runner.EditableInContext(ownerID, repoID) {
		ctx.NotFound()
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
		ctx.ServerErrorAuto(err)
		return
	}

	if err = actions_model.TaskList(tasks).LoadAttributes(ctx); err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	ctx.Data["Tasks"] = tasks
	pager := context.NewPagerBuilder(ctx).TotalCount(count).PerPageLimit(opts.PageSize).CurPage(opts.Page).Build()
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, runnerEditTemplates[rCtx.Kind])
}

func RunnersEditPost(ctx *context.Context) {
	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	runnerID := ctx.PathParamInt64("runnerid")
	ownerID := rCtx.OwnerID
	repoID := rCtx.RepoID
	redirectTo := rCtx.RedirectLink

	runner, err := actions_model.GetRunnerByID(ctx, runnerID)
	if err != nil {
		log.Warn("RunnerDetailsEditPost.GetRunnerByID failed: %v, url: %s", err, ctx.Req.URL)
		ctx.ServerErrorAuto(err)
		return
	}
	if !runner.EditableInContext(ownerID, repoID) {
		ctx.NotFound()
		return
	}

	form := web.GetForm[*forms.EditRunnerForm](ctx)
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
		ctx.ServerErrorAuto(err)
		return
	}

	ownerID := rCtx.OwnerID
	repoID := rCtx.RepoID
	redirectTo := rCtx.RedirectLink

	if _, err := actions_model.NewRunnerToken(ctx, ownerID, repoID); err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	audit.RecordScopedByID(ctx, ownerID, repoID, audit.RunnerToken)
	ctx.Flash.Success(ctx.Tr("actions.runners.reset_registration_token_success"))
	ctx.JSONRedirect(redirectTo)
}

// RunnerDeletePost response for deleting runner
func RunnerDeletePost(ctx *context.Context) {
	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	runner := findActionsRunner(ctx, rCtx)
	if ctx.Written() {
		return
	}

	if err := actions_model.DeleteRunner(ctx, runner.ID); err != nil {
		log.Warn("DeleteRunnerPost.UpdateRunner failed: %v, url: %s", err, ctx.Req.URL)
		ctx.Flash.Warning(ctx.Tr("actions.runners.delete_runner_failed"))

		ctx.JSONRedirect(rCtx.RedirectLink + url.PathEscape(ctx.PathParam("runnerid")))
		return
	}

	audit.RecordScopedByID(ctx, runner.OwnerID, runner.RepoID, audit.RunnerRemove, "runner", runner.Name)
	log.Info("DeleteRunnerPost success: %s", ctx.Req.URL)

	ctx.Flash.Success(ctx.Tr("actions.runners.delete_runner_success"))

	ctx.JSONRedirect(rCtx.RedirectLink)
}

func RunnerUpdatePost(ctx *context.Context) {
	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	runner := findActionsRunner(ctx, rCtx)
	if ctx.Written() {
		return
	}

	isDisabled := ctx.FormOptionalBool("disabled")
	if !isDisabled.Has() {
		ctx.HTTPError(http.StatusBadRequest, "missing 'disabled' parameter")
		return
	}

	successKey := "actions.runners.enable_runner_success"
	failedKey := "actions.runners.enable_runner_failed"
	if isDisabled.Value() {
		successKey = "actions.runners.disable_runner_success"
		failedKey = "actions.runners.disable_runner_failed"
	}

	if err := actions_model.SetRunnerDisabled(ctx, runner, isDisabled.Value()); err != nil {
		log.Warn("RunnerUpdatePost.SetRunnerDisabled failed: %v, url: %s", err, ctx.Req.URL)
		ctx.Flash.Error(ctx.Tr(failedKey))
		ctx.JSONRedirect("")
		return
	}
	audit.RecordScopedByID(ctx, runner.OwnerID, runner.RepoID, audit.RunnerUpdate, "runner", runner.Name, "disabled", isDisabled.Value())

	ctx.Flash.Success(ctx.Tr(successKey))
	ctx.JSONRedirect("")
}

// RunnerBulkActionPost performs a bulk action (delete/disable/enable) on multiple runners.
// Admin-only: route must be mounted inside the admin runners group; defense-in-depth check below.
func RunnerBulkActionPost(ctx *context.Context) {
	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	if !rCtx.IsAdmin() {
		ctx.HTTPError(http.StatusForbidden, "bulk actions are admin-only")
		return
	}
	// ATTENTION: it completely depends on the assumption that the doer is "site admin"
	// So it doesn't do extra permission check to the runner IDs
	// In the future, if you need to support such operation on non-admin pages, be careful!
	runnerIDs := ctx.FormStringInt64s("ids")
	if len(runnerIDs) == 0 {
		ctx.HTTPError(http.StatusBadRequest, "missing runner IDs")
		return
	}

	action := ctx.FormString("action")
	var successKey, failedKey string
	switch action {
	case "delete":
		successKey, failedKey = "actions.runners.delete_runner_success", "actions.runners.delete_runner_failed"
	case "disable":
		successKey, failedKey = "actions.runners.disable_runner_success", "actions.runners.disable_runner_failed"
	case "enable":
		successKey, failedKey = "actions.runners.enable_runner_success", "actions.runners.enable_runner_failed"
	default:
		ctx.HTTPError(http.StatusBadRequest, "invalid action")
		return
	}

	runners, err := db.Find[actions_model.ActionRunner](ctx, &actions_model.FindRunnerOptions{IDs: runnerIDs})
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	deleting, disabled := action == "delete", action == "disable"
	if err := db.WithTx(ctx, func(txCtx stdctx.Context) error {
		for _, runner := range runners {
			var err error
			if deleting {
				err = actions_model.DeleteRunner(txCtx, runner.ID)
			} else {
				err = actions_model.SetRunnerDisabled(txCtx, runner, disabled)
			}
			if err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		log.Warn("RunnerBulkActionPost.%s failed: %v, url: %s", action, err, ctx.Req.URL)
		ctx.Flash.Error(ctx.Tr(failedKey))
		ctx.JSONRedirect(rCtx.RedirectLink)
		return
	}

	for _, runner := range runners {
		if deleting {
			audit.RecordScopedByID(ctx, runner.OwnerID, runner.RepoID, audit.RunnerRemove, "runner", runner.Name)
		} else {
			audit.RecordScopedByID(ctx, runner.OwnerID, runner.RepoID, audit.RunnerUpdate, "runner", runner.Name, "disabled", disabled)
		}
	}

	ctx.Flash.Success(ctx.Tr(successKey))
	ctx.JSONRedirect(rCtx.RedirectLink)
}

func findActionsRunner(ctx *context.Context, rCtx *runnersCtx) *actions_model.ActionRunner {
	got, err := db.Find[actions_model.ActionRunner](ctx, &actions_model.FindRunnerOptions{
		IDs:     []int64{ctx.PathParamInt64("runnerid")},
		OwnerID: rCtx.OwnerID,
		RepoID:  rCtx.RepoID,
	})
	if err != nil {
		ctx.ServerErrorAuto(err)
		return nil
	} else if len(got) == 0 || !got[0].EditableInContext(rCtx.OwnerID, rCtx.RepoID) {
		ctx.NotFound()
		return nil
	}

	return got[0]
}
