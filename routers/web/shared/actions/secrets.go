// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"net/http"

	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	shared_secrets "gitea.dev/routers/web/shared/secrets"
	"gitea.dev/services/context"
)

const (
	// TODO: Separate secrets from runners when layout is ready
	tplRepoSecrets templates.TplName = "repo/settings/actions"
	tplOrgSecrets  templates.TplName = "org/settings/actions"
	tplUserSecrets templates.TplName = "user/settings/actions"
)

type secretsCtx struct {
	*settingsScope
	Owner           *user_model.User // audit records need the loaded owner or repo, not only its ID
	Repo            *repo_model.Repository
	SecretsTemplate templates.TplName
	RedirectLink    string
}

func getSecretsCtx(ctx *context.Context) (*secretsCtx, error) {
	scope, err := getSettingsScope(ctx)
	if err != nil {
		return nil, err
	}
	sCtx := &secretsCtx{settingsScope: scope, RedirectLink: scope.LinkPrefix + "/secrets"}
	switch {
	case scope.IsRepo:
		sCtx.Repo, sCtx.SecretsTemplate = ctx.Repo.Repository, tplRepoSecrets
	case scope.IsOrg:
		sCtx.Owner, sCtx.SecretsTemplate = ctx.ContextUser, tplOrgSecrets
	case scope.IsUser:
		sCtx.Owner, sCtx.SecretsTemplate = ctx.Doer, tplUserSecrets
	default:
		return nil, errors.New("getSecretsCtx: secrets are not managed at instance level")
	}
	return sCtx, nil
}

func Secrets(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("secrets.secrets")
	ctx.Data["PageType"] = "secrets"
	ctx.Data["PageIsSharedSettingsSecrets"] = true

	sCtx, err := getSecretsCtx(ctx)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	if sCtx.IsRepo {
		ctx.Data["DisableSSH"] = setting.SSH.Disabled
	}

	shared_secrets.SetSecretsContext(ctx, sCtx.BelongingScope)
	if ctx.Written() {
		return
	}
	ctx.HTML(http.StatusOK, sCtx.SecretsTemplate)
}

func SecretsPost(ctx *context.Context) {
	sCtx, err := getSecretsCtx(ctx)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	shared_secrets.PerformSecretsPost(
		ctx,
		sCtx.BelongingScope,
		sCtx.Owner,
		sCtx.Repo,
		sCtx.RedirectLink,
	)
}

func SecretsDelete(ctx *context.Context) {
	sCtx, err := getSecretsCtx(ctx)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	shared_secrets.PerformSecretsDelete(
		ctx,
		sCtx.BelongingScope,
		sCtx.Owner,
		sCtx.Repo,
		sCtx.RedirectLink,
	)
}
