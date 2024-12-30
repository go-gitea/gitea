// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"net/http"
	"net/url"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	actions_shared "code.gitea.io/gitea/routers/web/shared/actions"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/context"
)

const (
	// TODO: Separate secrets from runners when layout is ready
	tplRepoRunners     templates.TplName = "repo/settings/actions"
	tplOrgRunners      templates.TplName = "org/settings/actions"
	tplAdminRunners    templates.TplName = "admin/actions"
	tplUserRunners     templates.TplName = "user/settings/actions"
	tplRepoRunnerEdit  templates.TplName = "repo/settings/runner_edit"
	tplOrgRunnerEdit   templates.TplName = "org/settings/runners_edit"
	tplAdminRunnerEdit templates.TplName = "admin/runners/edit"
	tplUserRunnerEdit  templates.TplName = "user/settings/runner_edit"
)

type runnersCtx struct {
	OwnerID            int64
	RepoID             int64
	IsRepo             bool
	IsOrg              bool
	IsAdmin            bool
	IsUser             bool
	RunnersTemplate    templates.TplName
	RunnerEditTemplate templates.TplName
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
	actions_shared.RunnersList(ctx, opts)

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

	actions_shared.RunnerDetails(ctx, page,
		ctx.PathParamInt64("runnerid"), rCtx.OwnerID, rCtx.RepoID,
	)
	ctx.HTML(http.StatusOK, rCtx.RunnerEditTemplate)
}

func RunnersEditPost(ctx *context.Context) {
	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerError("getRunnersCtx", err)
		return
	}
	actions_shared.RunnerDetailsEditPost(ctx, ctx.PathParamInt64("runnerid"),
		rCtx.OwnerID, rCtx.RepoID,
		rCtx.RedirectLink+url.PathEscape(ctx.PathParam("runnerid")))
}

func ResetRunnerRegistrationToken(ctx *context.Context) {
	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerError("getRunnersCtx", err)
		return
	}
	actions_shared.RunnerResetRegistrationToken(ctx, rCtx.OwnerID, rCtx.RepoID, rCtx.RedirectLink)
}

// RunnerDeletePost response for deleting runner
func RunnerDeletePost(ctx *context.Context) {
	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerError("getRunnersCtx", err)
		return
	}
	actions_shared.RunnerDeletePost(ctx, ctx.PathParamInt64("runnerid"), rCtx.RedirectLink, rCtx.RedirectLink+url.PathEscape(ctx.PathParam("runnerid")))
}

func RedirectToDefaultSetting(ctx *context.Context) {
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/actions/runners")
}
