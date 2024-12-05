// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"net/http"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/setting"
	shared "code.gitea.io/gitea/routers/web/shared/secrets"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/context"
)

const (
	// TODO: Separate secrets from runners when layout is ready
	tplRepoSecrets base.TplName = "repo/settings/actions"
	tplOrgSecrets  base.TplName = "org/settings/actions"
	tplUserSecrets base.TplName = "user/settings/actions"
)

type secretsCtx struct {
	Owner           *user_model.User
	Repo            *repo_model.Repository
	IsRepo          bool
	IsOrg           bool
	IsUser          bool
	SecretsTemplate base.TplName
	RedirectLink    string
}

func getSecretsCtx(ctx *context.Context) (*secretsCtx, error) {
	if ctx.Data["PageIsRepoSettings"] == true {
		return &secretsCtx{
			Owner:           nil,
			Repo:            ctx.Repo.Repository,
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
			Owner:           ctx.ContextUser,
			Repo:            nil,
			IsOrg:           true,
			SecretsTemplate: tplOrgSecrets,
			RedirectLink:    ctx.Org.OrgLink + "/settings/actions/secrets",
		}, nil
	}

	if ctx.Data["PageIsUserSettings"] == true {
		return &secretsCtx{
			Owner:           ctx.Doer,
			Repo:            nil,
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

	shared.SetSecretsContext(ctx, sCtx.Owner, sCtx.Repo)
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
		ctx.Doer,
		sCtx.Owner,
		sCtx.Repo,
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
		ctx.Doer,
		sCtx.Owner,
		sCtx.Repo,
		sCtx.RedirectLink,
	)
}
