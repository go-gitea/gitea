// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// WIP RequireAction

package setting

import (
	"errors"
	"net/http"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"

	//"code.gitea.io/gitea/modules/setting"
	shared "code.gitea.io/gitea/routers/web/shared/actions"
	//shared_user "code.gitea.io/gitea/routers/web/shared/user"
	actions_model "code.gitea.io/gitea/models/actions"
	actions_shared "code.gitea.io/gitea/routers/web/shared/actions"
)

const (
	// let start with org WIP
	//tplRepoReqireActions base.TplName = "repo/settings/actions"
	//tplUserRequireActions  base.TplName = "user/settings/actions"
	//tplAdminRequireActions base.TplName = "admin/actions"
	tplOrgRequireActions base.TplName = "org/settings/actions"
)

type requireActionsCtx struct {
	OrgID                  int64
	IsOrg                  bool
	RequireActionsTemplate base.TplName
	RedirectLink           string
}

func getRequireActionsCtx(ctx *context.Context) (*requireActionsCtx, error) {
	if ctx.Data["PageIsOrgSettings"] == true {
		/*
			err := shared_user.LoadHeaderCount(ctx)
			if err != nil {
				ctx.ServerError("LoadHeaderCount", err)
				return nil, nil
			}
		*/
		return &requireActionsCtx{
			OrgID:                  ctx.Org.Organization.ID,
			IsOrg:                  true,
			RequireActionsTemplate: tplOrgRequireActions,
			RedirectLink:           ctx.Org.OrgLink + "/settings/actions/require_action",
		}, nil
	}
	/*
		if ctx.Data["PageIsRepoSettings"] == true {
			return &requireActionsCtx{
				OwnerID:                0,
				RepoID:                 ctx.Repo.Repository.ID,
				RequireActionsTemplate: tplRepoReqireActions,
				// RedirectLink:           ctx.Repo.RepoLink + "/settings/actions/require_action_list",
				RedirectLink: ctx.Repo.RepoLink + "/settings/actions/require_action",
			}, nil
		}

		if ctx.Data["PageIsUserSettings"] == true {
				return &requireActionsCtx{
					OwnerID:                ctx.Doer.ID,
					RepoID:                 0,
					RequireActionsTemplate: tplUserVariables,
					RedirectLink:           setting.AppSubURL + "/user/settings/actions/require_action",
				}, nil
			}

			if ctx.Data["PageIsAdmin"] == true {
				return &requireActionsCtx{
					OwnerID:                0,
					RepoID:                 0,
					IsGlobal:               true,
					RequireActionsTemplate: tplAdminVariables,
					RedirectLink:           setting.AppSubURL + "/admin/actions/require_action",
				}, nil
			}
	*/
	return nil, errors.New("unable to set Require Actions context")
}

func RequireActionsList(ctx *context.Context) {
	ctx.Data["ActionsTitle"] = ctx.Tr("actions.requires")
	ctx.Data["PageType"] = "require_actions"
	ctx.Data["PageIsSharedSettingsRequireActions"] = true

	vCtx, err := getRequireActionsCtx(ctx)
	if err != nil {
		ctx.ServerError("getRequireActionsCtx", err)
		return
	}

	shared.SetRequireActionsContext(ctx)
	if ctx.Written() {
		return
	}
	opts := actions_model.FindRequireActionOptions{
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: 10,
		},
		OrgID: ctx.Org.Organization.ID,
	}
	actions_shared.RequireActionsList(ctx, opts)
	ctx.HTML(http.StatusOK, vCtx.RequireActionsTemplate)
}

func RequireActionsCreate(ctx *context.Context) {
	log.Trace("Require Action Trace")
	vCtx, err := getRequireActionsCtx(ctx)
	if err != nil {
		ctx.ServerError("getRequireActionsCtx", err)
		return
	}

	if ctx.HasError() { // form binding validation error
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	shared.CreateRequireAction(
		ctx,
		vCtx.OrgID,
		vCtx.RedirectLink,
	)
}

/*
func RequireActionsUpdate(ctx *context.Context) {
	vCtx, err := getRequireActionsCtx(ctx)
	if err != nil {
		ctx.ServerError("getRequireActionsCtx", err)
		return
	}
	if ctx.HasError() { // form binding validation error
		ctx.JSONError(ctx.GetErrMsg())
		return
	}
	shared.UpdateRequireAction(ctx, vCtx.RedirectLink)
}

func RequireActionsUpdatePost(ctx *context.Context) {
	vCtx, err := getRequireActionsCtx(ctx)
	if err != nil {
		ctx.ServerError("getRequireActionsCtx", err)
		return
	}
	shared.UpdateRequireAction(ctx, vCtx.RedirectLink)
}

func RequireActionsDelete(ctx *context.Context) {
	vCtx, err := getRequireActionsCtx(ctx)
	if err != nil {
		ctx.ServerError("getRequireActionsCtx", err)
		return
	}
	shared.DeleteRequireAction(ctx, vCtx.RedirectLink)
}
func RequireActionsDeletePost(ctx *context.Context) {
	vCtx, err := getRequireActionsCtx(ctx)
	if err != nil {
		ctx.ServerError("getRequireActionsCtx", err)
		return
	}
	shared.DeleteRequireAction(ctx, vCtx.RedirectLink)
}
*/
