// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"net/http"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	shared "code.gitea.io/gitea/routers/web/shared/secrets"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/context"
)

const (
	// TODO: Separate secrets from runners when layout is ready
	tplRepoSecrets templates.TplName = "repo/settings/actions"
	tplOrgSecrets  templates.TplName = "org/settings/actions"
	tplUserSecrets templates.TplName = "user/settings/actions"
)

type secretsCtx struct {
	OwnerID         int64
	RepoID          int64
	IsRepo          bool
	IsOrg           bool
	IsUser          bool
	SecretsTemplate templates.TplName
	RedirectLink    string
}

func getSecretsCtx(ctx *context.Context) (*secretsCtx, error) {
	if ctx.Data["PageIsRepoSettings"] == true {
		return &secretsCtx{
			OwnerID:         0,
			RepoID:          ctx.Repo.Repository.ID,
			IsRepo:          true,
			SecretsTemplate: tplRepoSecrets,
			RedirectLink:    ctx.Repo.RepoLink + "/settings/actions/secrets",
		}, nil
	}

	if ctx.Data["PageIsOrgSettings"] == true {
		err := shared_user.LoadHeaderCount(ctx)
		if err != nil {
			ctx.ServerError("LoadHeaderCount", err)
			return nil, nil
		}
		return &secretsCtx{
			OwnerID:         ctx.ContextUser.ID,
			RepoID:          0,
			IsOrg:           true,
			SecretsTemplate: tplOrgSecrets,
			RedirectLink:    ctx.Org.OrgLink + "/settings/actions/secrets",
		}, nil
	}

	if ctx.Data["PageIsUserSettings"] == true {
		return &secretsCtx{
			OwnerID:         ctx.Doer.ID,
			RepoID:          0,
			IsUser:          true,
			SecretsTemplate: tplUserSecrets,
			RedirectLink:    setting.AppSubURL + "/user/settings/actions/secrets",
		}, nil
	}

	return nil, errors.New("unable to set Secrets context")
}

func Secrets(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.actions")
	ctx.Data["PageType"] = "secrets"
	ctx.Data["PageIsSharedSettingsSecrets"] = true
	ctx.Data["UserDisabledFeatures"] = user_model.DisabledFeaturesWithLoginType(ctx.Doer)

	sCtx, err := getSecretsCtx(ctx)
	if err != nil {
		ctx.ServerError("getSecretsCtx", err)
		return
	}

	if sCtx.IsRepo {
		ctx.Data["DisableSSH"] = setting.SSH.Disabled
	}

	shared.SetSecretsContext(ctx, sCtx.OwnerID, sCtx.RepoID)
	if ctx.Written() {
		return
	}
	ctx.HTML(http.StatusOK, sCtx.SecretsTemplate)
}

func SecretsPost(ctx *context.Context) {
	sCtx, err := getSecretsCtx(ctx)
	if err != nil {
		ctx.ServerError("getSecretsCtx", err)
		return
	}

	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	shared.PerformSecretsPost(
		ctx,
		sCtx.OwnerID,
		sCtx.RepoID,
		sCtx.RedirectLink,
	)
}

func SecretsDelete(ctx *context.Context) {
	sCtx, err := getSecretsCtx(ctx)
	if err != nil {
		ctx.ServerError("getSecretsCtx", err)
		return
	}
	shared.PerformSecretsDelete(
		ctx,
		sCtx.OwnerID,
		sCtx.RepoID,
		sCtx.RedirectLink,
	)
}
