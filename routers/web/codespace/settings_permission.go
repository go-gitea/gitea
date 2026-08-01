// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"errors"
	"net/http"

	"gitea.dev/models/perm"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	codespace_service "gitea.dev/services/codespace"
	"gitea.dev/services/context"
)

const tplUserCodespacePermissions templates.TplName = "codespace/user_permissions"

// UserPermissionSettings renders the current user's remembered repository grants.
func UserPermissionSettings(ctx *context.Context) {
	authorizations, err := codespace_service.ListPermissionAuthorizations(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("ListPermissionAuthorizations", err)
		return
	}
	ctx.Data["Title"] = ctx.Tr("codespace.permissions")
	ctx.Data["PageIsCodespacePermissions"] = true
	ctx.Data["PermissionAuthorizations"] = authorizations
	ctx.HTML(http.StatusOK, tplUserCodespacePermissions)
}

// UserPermissionSettingsPost reduces or revokes a remembered repository grant.
func UserPermissionSettingsPost(ctx *context.Context) {
	authorizationID := ctx.FormInt64("authorization_id")
	var err error
	switch ctx.FormString("action") {
	case "revoke":
		err = codespace_service.RevokePermissionAuthorization(ctx, ctx.Doer.ID, authorizationID)
	case "reduce":
		var mode perm.AccessMode
		switch ctx.FormString("mode") {
		case "read":
			mode = perm.AccessModeRead
		case "none":
			mode = perm.AccessModeNone
		default:
			err = codespace_service.ErrPermissionReductionInvalid
		}
		if err == nil {
			err = codespace_service.ReducePermissionRepository(ctx, ctx.Doer.ID, authorizationID, ctx.FormInt64("rule_id"), mode)
		}
	default:
		err = codespace_service.ErrPermissionReductionInvalid
	}
	if err != nil {
		if errors.Is(err, codespace_service.ErrPermissionAuthorizationNotFound) {
			ctx.NotFound(nil)
			return
		}
		ctx.Flash.Error(ctx.Tr("codespace.error.invalid_request"))
	} else {
		ctx.Flash.Success(ctx.Tr("codespace.permissions_updated"))
	}
	ctx.Redirect(setting.AppSubURL+"/user/settings/codespaces/permissions", http.StatusSeeOther)
}
