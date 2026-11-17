// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"errors"
	"net/http"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	codespace_service "gitea.dev/services/codespace"
	"gitea.dev/services/context"
)

const tplUserCodespaceSecrets templates.TplName = "codespace/user_secrets"

// UserSecretSettings renders the current user's Codespace environment secrets.
func UserSecretSettings(ctx *context.Context) {
	secrets, err := codespace_service.ListUserSecrets(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	ctx.Data["Title"] = ctx.Tr("codespace.secrets")
	ctx.Data["PageIsCodespaceSecrets"] = true
	ctx.Data["CodespaceSecrets"] = secrets
	ctx.HTML(http.StatusOK, tplUserCodespaceSecrets)
}

// UserSecretSettingsPost creates a user-owned Codespace secret.
func UserSecretSettingsPost(ctx *context.Context) {
	err := codespace_service.CreateUserSecret(ctx, ctx.Doer, ctx.FormString("name"), ctx.FormString("value"))
	if err != nil {
		handleSecretSettingsError(ctx, err)
		return
	}
	ctx.Flash.Success(ctx.Tr("codespace.secret_updated"))
	ctx.JSONRedirect(setting.AppSubURL + "/user/settings/codespaces/secrets")
}

// UserSecretSettingsValue replaces one secret value.
func UserSecretSettingsValue(ctx *context.Context) {
	err := codespace_service.UpdateUserSecretValue(ctx, ctx.Doer.ID, ctx.PathParamInt64("secret_id"), ctx.FormString("value"))
	if err != nil {
		handleSecretSettingsError(ctx, err)
		return
	}
	ctx.Flash.Success(ctx.Tr("codespace.secret_updated"))
	ctx.JSONRedirect(setting.AppSubURL + "/user/settings/codespaces/secrets")
}

// UserSecretSettingsDelete deletes one saved secret.
func UserSecretSettingsDelete(ctx *context.Context) {
	err := codespace_service.DeleteUserSecret(ctx, ctx.Doer.ID, ctx.PathParamInt64("secret_id"))
	if err != nil {
		handleSecretSettingsError(ctx, err)
		return
	}
	ctx.Flash.Success(ctx.Tr("codespace.secret_updated"))
	ctx.JSONRedirect(setting.AppSubURL + "/user/settings/codespaces/secrets")
}

func handleSecretSettingsError(ctx *context.Context, err error) {
	switch {
	case errors.Is(err, codespace_service.ErrUserSecretNotFound):
		ctx.JSONErrorNotFound()
	case errors.Is(err, codespace_service.ErrUserSecretNameInvalid):
		ctx.JSONError(ctx.Tr("codespace.secret_name_invalid"))
	case errors.Is(err, codespace_service.ErrUserSecretNameConflict):
		ctx.JSONError(ctx.Tr("codespace.secret_name_conflict"))
	case errors.Is(err, codespace_service.ErrUserSecretValueInvalid):
		ctx.JSONError(ctx.Tr("codespace.secret_value_invalid"))
	case errors.Is(err, codespace_service.ErrUserSecretCountLimit):
		ctx.JSONError(ctx.Tr("codespace.secret_count_limit"))
	case errors.Is(err, util.ErrInvalidArgument):
		ctx.JSONError(ctx.Tr("codespace.secret_update_failed"))
	default:
		ctx.JSONErrorAuto(err)
	}
}
