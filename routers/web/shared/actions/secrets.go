// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"net/http"

	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	shared_secrets "gitea.dev/routers/web/shared/secrets"
	"gitea.dev/services/context"
)

func getSecretsScope(ctx *context.Context) (*settingsScope, error) {
	scope, err := getSettingsScope(ctx)
	if err != nil {
		return nil, err
	}
	if scope.IsAdmin() {
		return nil, errors.New("getSecretsScope: secrets are not managed at instance level")
	}
	return scope, nil
}

func secretAuditTarget(ctx *context.Context, scope *settingsScope) (*user_model.User, *repo_model.Repository) {
	switch scope.Kind {
	case settingsScopeRepo:
		return nil, ctx.Repo.Repository
	case settingsScopeOrg:
		return ctx.ContextUser, nil
	default:
		return ctx.Doer, nil
	}
}

func Secrets(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("secrets.secrets")
	ctx.Data["PageType"] = "secrets"
	ctx.Data["PageIsSharedSettingsSecrets"] = true

	scope, err := getSecretsScope(ctx)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	shared_secrets.SetSecretsContext(ctx, scope.BelongingScope)
	if !ctx.Written() {
		ctx.HTML(http.StatusOK, scope.Template)
	}
}

func SecretsPost(ctx *context.Context) {
	scope, err := getSecretsScope(ctx)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	owner, repo := secretAuditTarget(ctx, scope)
	shared_secrets.PerformSecretsPost(ctx, scope.BelongingScope, owner, repo, scope.LinkPrefix+"/secrets")
}

func SecretsDelete(ctx *context.Context) {
	scope, err := getSecretsScope(ctx)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	owner, repo := secretAuditTarget(ctx, scope)
	shared_secrets.PerformSecretsDelete(ctx, scope.BelongingScope, owner, repo, scope.LinkPrefix+"/secrets")
}
