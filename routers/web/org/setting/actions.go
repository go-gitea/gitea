// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/services/context"
)

const (
	tplSettingsActionsGeneral base.TplName = "org/settings/actions_general"
)

// ActionsGeneral renders the actions general settings page
func ActionsGeneral(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.actions")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsOrgSettingsActions"] = true

	// Load Org Actions Config
	actionsCfg, err := actions_model.GetOrgActionsConfig(ctx, ctx.Org.Organization.ID)
	if err != nil {
		ctx.ServerError("GetOrgActionsConfig", err)
		return
	}

	ctx.Data["TokenPermissionMode"] = actionsCfg.GetTokenPermissionMode()
	ctx.Data["TokenPermissionModePermissive"] = repo_model.ActionsTokenPermissionModePermissive
	ctx.Data["TokenPermissionModeRestricted"] = repo_model.ActionsTokenPermissionModeRestricted
	ctx.Data["TokenPermissionModeCustom"] = repo_model.ActionsTokenPermissionModeCustom
	ctx.Data["DefaultTokenPermissions"] = actionsCfg.GetEffectiveTokenPermissions(false)

	ctx.Data["AllowCrossRepoAccess"] = actionsCfg.AllowCrossRepoAccess

	ctx.HTML(http.StatusOK, tplSettingsActionsGeneral)
}

// ActionsGeneralPost responses for actions general settings page
func ActionsGeneralPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.actions")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsOrgSettingsActions"] = true

	actionsCfg, err := actions_model.GetOrgActionsConfig(ctx, ctx.Org.Organization.ID)
	if err != nil {
		ctx.ServerError("GetOrgActionsConfig", err)
		return
	}

	// Update Token Permission Mode
	permissionMode := repo_model.ActionsTokenPermissionMode(ctx.FormString("token_permission_mode"))
	if permissionMode == repo_model.ActionsTokenPermissionModeRestricted ||
		permissionMode == repo_model.ActionsTokenPermissionModePermissive ||
		permissionMode == repo_model.ActionsTokenPermissionModeCustom {
		actionsCfg.TokenPermissionMode = permissionMode
	}

	if actionsCfg.TokenPermissionMode == repo_model.ActionsTokenPermissionModeCustom {
		parsePerm := func(name string) perm.AccessMode {
			if ctx.FormBool(name + "_write") {
				return perm.AccessModeWrite
			}
			if ctx.FormBool(name + "_read") {
				return perm.AccessModeRead
			}
			return perm.AccessModeNone
		}

		actionsCfg.DefaultTokenPermissions = &repo_model.ActionsTokenPermissions{
			Actions:      parsePerm("actions"),
			Contents:     parsePerm("contents"),
			Issues:       parsePerm("issues"),
			Packages:     parsePerm("packages"),
			PullRequests: parsePerm("pull_requests"),
			Wiki:         parsePerm("wiki"),
		}
	} else {
		actionsCfg.DefaultTokenPermissions = nil
	}

	// Update Cross-Repo Access
	actionsCfg.AllowCrossRepoAccess = ctx.FormBool("allow_cross_repo_access")

	if err := actions_model.SetOrgActionsConfig(ctx, ctx.Org.Organization.ID, actionsCfg); err != nil {
		ctx.ServerError("SetOrgActionsConfig", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("org.settings.update_setting_success"))
	ctx.Redirect(ctx.Org.OrgLink + "/settings/actions")
}
