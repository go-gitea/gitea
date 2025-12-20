// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
)

const (
	tplSettingsActionsGeneral templates.TplName = "org/settings/actions_general"
)

// ActionsGeneral renders the actions general settings page
func ActionsGeneral(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.actions")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsOrgSettingsActionsGeneral"] = true

	// Load Org Actions Config
	actionsCfg, err := actions_model.GetOrgActionsConfig(ctx, ctx.Org.Organization.AsUser().ID)
	if err != nil {
		ctx.ServerError("GetOrgActionsConfig", err)
		return
	}

	ctx.Data["TokenPermissionMode"] = actionsCfg.GetTokenPermissionMode()
	ctx.Data["TokenPermissionModePermissive"] = repo_model.ActionsTokenPermissionModePermissive
	ctx.Data["TokenPermissionModeRestricted"] = repo_model.ActionsTokenPermissionModeRestricted
	ctx.Data["TokenPermissionModeCustom"] = repo_model.ActionsTokenPermissionModeCustom
	ctx.Data["DefaultTokenPermissions"] = actionsCfg.GetEffectiveTokenPermissions(false)
	ctx.Data["MaxTokenPermissions"] = actionsCfg.GetMaxTokenPermissions()

	ctx.Data["AllowCrossRepoAccess"] = actionsCfg.AllowCrossRepoAccess

	ctx.HTML(http.StatusOK, tplSettingsActionsGeneral)
}

// ActionsGeneralPost responses for actions general settings page
func ActionsGeneralPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.actions")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsOrgSettingsActions"] = true

	actionsCfg, err := actions_model.GetOrgActionsConfig(ctx, ctx.Org.Organization.AsUser().ID)
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
		// Custom mode uses radio buttons for each permission scope
		parsePerm := func(name string) perm.AccessMode {
			value := ctx.FormString(name)
			switch value {
			case "write":
				return perm.AccessModeWrite
			case "read":
				return perm.AccessModeRead
			default:
				return perm.AccessModeNone
			}
		}

		actionsCfg.DefaultTokenPermissions = &repo_model.ActionsTokenPermissions{
			Actions:      parsePerm("perm_actions"),
			Contents:     parsePerm("perm_contents"),
			Issues:       parsePerm("perm_issues"),
			Packages:     parsePerm("perm_packages"),
			PullRequests: parsePerm("perm_pull_requests"),
			Wiki:         parsePerm("perm_wiki"),
		}
	} else {
		actionsCfg.DefaultTokenPermissions = nil
	}

	// Update Maximum Permissions (radio buttons: none/read/write)
	parseMaxPerm := func(name string) perm.AccessMode {
		value := ctx.FormString("max_" + name)
		switch value {
		case "write":
			return perm.AccessModeWrite
		case "read":
			return perm.AccessModeRead
		default:
			return perm.AccessModeNone
		}
	}

	actionsCfg.MaxTokenPermissions = &repo_model.ActionsTokenPermissions{
		Actions:      parseMaxPerm("actions"),
		Contents:     parseMaxPerm("contents"),
		Issues:       parseMaxPerm("issues"),
		Packages:     parseMaxPerm("packages"),
		PullRequests: parseMaxPerm("pull_requests"),
		Wiki:         parseMaxPerm("wiki"),
	}

	// Update Cross-Repo Access
	actionsCfg.AllowCrossRepoAccess = ctx.FormBool("allow_cross_repo_access")

	if err := actions_model.SetOrgActionsConfig(ctx, ctx.Org.Organization.AsUser().ID, actionsCfg); err != nil {
		ctx.ServerError("SetOrgActionsConfig", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("org.settings.update_setting_success"))
	ctx.Redirect(ctx.Org.OrgLink + "/settings/actions")
}
