// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"net/http"
	"net/url"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	actions_shared "code.gitea.io/gitea/routers/web/shared/actions"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
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
	Owner              *user_model.User
	RepoID             int64
	Repo               *repo_model.Repository
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
			Repo:               ctx.Repo.Repository,
			OwnerID:            0,
			Owner:              nil,
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
			Repo:               nil,
			OwnerID:            ctx.Org.Organization.ID,
			Owner:              ctx.Org.Organization.AsUser(),
			IsOrg:              true,
			RunnersTemplate:    tplOrgRunners,
			RunnerEditTemplate: tplOrgRunnerEdit,
			RedirectLink:       ctx.Org.OrgLink + "/settings/actions/runners/",
		}, nil
	}

	if ctx.Data["PageIsAdmin"] == true {
		return &runnersCtx{
			RepoID:             0,
			Repo:               nil,
			OwnerID:            0,
			Owner:              nil,
			IsAdmin:            true,
			RunnersTemplate:    tplAdminRunners,
			RunnerEditTemplate: tplAdminRunnerEdit,
			RedirectLink:       setting.AppSubURL + "/admin/actions/runners/",
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
		opts.Repo = rCtx.Repo
		opts.WithAvailable = true
	} else if rCtx.IsOrg || rCtx.IsUser {
		opts.OwnerID = rCtx.OwnerID
		opts.Owner = rCtx.Owner
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
		ctx.ParamsInt64(":runnerid"), rCtx.Owner, rCtx.Repo,
	)
	ctx.HTML(http.StatusOK, rCtx.RunnerEditTemplate)
}

func RunnersEditPost(ctx *context.Context) {
	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerError("getRunnersCtx", err)
		return
	}
	actions_shared.RunnerDetailsEditPost(ctx, ctx.ParamsInt64(":runnerid"),
		rCtx.RedirectLink+url.PathEscape(ctx.Params(":runnerid")),
		rCtx.Owner, rCtx.Repo)
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
	actions_shared.RunnerDeletePost(ctx, ctx.ParamsInt64(":runnerid"), rCtx.RedirectLink, rCtx.RedirectLink+url.PathEscape(ctx.Params(":runnerid")))
}

func RedirectToDefaultSetting(ctx *context.Context) {
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/actions/runners")
}
