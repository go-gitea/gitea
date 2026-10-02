// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"errors"
	"net/http"
	"strconv"

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
		ctx.ServerError("ListUserSecrets", err)
		return
	}
	ctx.Data["Title"] = ctx.Tr("codespace.secrets")
	ctx.Data["PageIsCodespaceSecrets"] = true
	ctx.Data["CodespaceSecrets"] = secrets
	ctx.HTML(http.StatusOK, tplUserCodespaceSecrets)
}

// UserSecretSettingsPost creates a user-owned Codespace secret.
func UserSecretSettingsPost(ctx *context.Context) {
	repoIDs, err := codespaceSecretRepositoryIDsFromForm(ctx)
	if err == nil {
		err = codespace_service.CreateUserSecret(ctx, ctx.Doer, ctx.FormString("name"), ctx.FormString("value"), ctx.FormBool("all_repositories"), repoIDs)
	}
	if err != nil {
		codespaceSettingsJSONError(ctx, err)
		return
	}
	ctx.Flash.Success(ctx.Tr("codespace.secret_updated"))
	ctx.JSONRedirect(setting.AppSubURL + "/user/settings/codespaces/secrets")
}

// UserSecretSettingsValue replaces one secret value.
func UserSecretSettingsValue(ctx *context.Context) {
	err := codespace_service.UpdateUserSecretValue(ctx, ctx.Doer.ID, ctx.PathParamInt64("secret_id"), ctx.FormString("value"))
	if err != nil {
		codespaceSettingsJSONError(ctx, err)
		return
	}
	ctx.Flash.Success(ctx.Tr("codespace.secret_updated"))
	ctx.JSONRedirect(setting.AppSubURL + "/user/settings/codespaces/secrets")
}

// UserSecretSettingsAccess replaces one secret repository scope.
func UserSecretSettingsAccess(ctx *context.Context) {
	repoIDs, err := codespaceSecretRepositoryIDsFromForm(ctx)
	if err == nil {
		err = codespace_service.UpdateUserSecretRepositoryAccess(ctx, ctx.Doer, ctx.PathParamInt64("secret_id"), ctx.FormBool("all_repositories"), repoIDs)
	}
	if err != nil {
		codespaceSettingsJSONError(ctx, err)
		return
	}
	ctx.Flash.Success(ctx.Tr("codespace.secret_updated"))
	ctx.JSONRedirect(setting.AppSubURL + "/user/settings/codespaces/secrets")
}

// UserSecretSettingsDelete deletes one secret and its selected repository scope.
func UserSecretSettingsDelete(ctx *context.Context) {
	err := codespace_service.DeleteUserSecret(ctx, ctx.Doer.ID, ctx.PathParamInt64("secret_id"))
	if err != nil {
		codespaceSettingsJSONError(ctx, err)
		return
	}
	ctx.Flash.Success(ctx.Tr("codespace.secret_updated"))
	ctx.JSONRedirect(setting.AppSubURL + "/user/settings/codespaces/secrets")
}

// UserSecretRepositorySearch returns repositories the current user can grant to a personal secret.
func UserSecretRepositorySearch(ctx *context.Context) {
	repositories, err := codespace_service.SearchWritableSecretRepositories(ctx, ctx.Doer, ctx.FormTrim("q"))
	if err != nil {
		ctx.ServerError("SearchWritableSecretRepositories", err)
		return
	}
	type repositoryResult struct {
		ID       int64  `json:"id"`
		FullName string `json:"full_name"`
	}
	result := make([]repositoryResult, 0, len(repositories))
	for _, repo := range repositories {
		result = append(result, repositoryResult{ID: repo.ID, FullName: repo.FullName()})
	}
	ctx.JSON(http.StatusOK, map[string]any{"data": result})
}

func codespaceSecretRepositoryIDsFromForm(ctx *context.Context) ([]int64, error) {
	values := ctx.FormStrings("repository_ids")
	result := make([]int64, 0, len(values))
	for _, value := range values {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			return nil, util.NewInvalidArgumentErrorf("invalid repository")
		}
		result = append(result, id)
	}
	return result, nil
}

func codespaceSettingsJSONError(ctx *context.Context, err error) {
	switch {
	case errors.Is(err, codespace_service.ErrUserSecretNotFound):
		ctx.JSONErrorNotFound()
	case errors.Is(err, util.ErrPermissionDenied):
		ctx.JSONError(ctx.Tr("codespace.secret_repository_permission_required"))
	case errors.Is(err, codespace_service.ErrUserSecretNameInvalid):
		ctx.JSONError(ctx.Tr("codespace.secret_name_invalid"))
	case errors.Is(err, codespace_service.ErrUserSecretNameConflict):
		ctx.JSONError(ctx.Tr("codespace.secret_name_conflict"))
	case errors.Is(err, codespace_service.ErrUserSecretValueInvalid):
		ctx.JSONError(ctx.Tr("codespace.secret_value_invalid"))
	case errors.Is(err, codespace_service.ErrUserSecretCountLimit):
		ctx.JSONError(ctx.Tr("codespace.secret_count_limit"))
	case errors.Is(err, codespace_service.ErrUserSecretSizeLimit):
		ctx.JSONError(ctx.Tr("codespace.secret_size_limit"))
	case errors.Is(err, util.ErrInvalidArgument):
		ctx.JSONError(ctx.Tr("codespace.secret_update_failed"))
	default:
		ctx.ServerError("UpdateCodespaceSettings", err)
	}
}
