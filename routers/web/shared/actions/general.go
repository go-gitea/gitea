// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"net/http"
	"slices"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
)

const (
	tplOrgSettingsActionsGeneral  templates.TplName = "org/settings/actions_general"
	tplUserSettingsActionsGeneral templates.TplName = "user/settings/actions_general"
)

// ParseMaxTokenPermissions parses the maximum token permissions from form values
func ParseMaxTokenPermissions(ctx *context.Context) *repo_model.ActionsTokenPermissions {
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

	return &repo_model.ActionsTokenPermissions{
		Code:         parseMaxPerm("code"),
		Issues:       parseMaxPerm("issues"),
		Packages:     parseMaxPerm("packages"),
		PullRequests: parseMaxPerm("pull_requests"),
		Wiki:         parseMaxPerm("wiki"),
		Actions:      parseMaxPerm("actions"),
		Releases:     parseMaxPerm("releases"),
		Projects:     parseMaxPerm("projects"),
	}
}

// GeneralSettings renders the actions general settings page
func GeneralSettings(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.actions")

	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerError("getRunnersCtx", err)
		return
	}

	if rCtx.IsOrg {
		ctx.Data["PageIsOrgSettings"] = true
		ctx.Data["PageIsOrgSettingsActionsGeneral"] = true
	} else if rCtx.IsUser {
		ctx.Data["PageIsUserSettings"] = true
		ctx.Data["PageIsUserSettingsActionsGeneral"] = true
	} else {
		ctx.NotFound(nil)
		return
	}

	// Load User/Org Actions Config
	actionsCfg, err := actions_model.GetUserActionsConfig(ctx, rCtx.OwnerID)
	if err != nil {
		ctx.ServerError("GetUserActionsConfig", err)
		return
	}

	ctx.Data["TokenPermissionMode"] = actionsCfg.TokenPermissionMode
	ctx.Data["TokenPermissionModePermissive"] = repo_model.ActionsTokenPermissionModePermissive
	ctx.Data["TokenPermissionModeRestricted"] = repo_model.ActionsTokenPermissionModeRestricted
	ctx.Data["MaxTokenPermissions"] = actionsCfg.GetMaxTokenPermissions()
	ctx.Data["EnableMaxTokenPermissions"] = actionsCfg.MaxTokenPermissions != nil

	ctx.Data["CrossRepoMode"] = actionsCfg.CrossRepoMode
	ctx.Data["ActionsCrossRepoModeNone"] = repo_model.ActionsCrossRepoModeNone
	ctx.Data["ActionsCrossRepoModeSelected"] = repo_model.ActionsCrossRepoModeSelected

	// Load Allowed Repositories
	allowedRepos, err := repo_model.GetOwnerRepositoriesByIDs(ctx, rCtx.OwnerID, actionsCfg.AllowedCrossRepoIDs)
	if err != nil {
		ctx.ServerError("GetOwnerRepositoriesByIDs", err)
		return
	}

	ctx.Data["AllowedRepos"] = allowedRepos
	ctx.Data["OwnerID"] = rCtx.OwnerID

	if rCtx.IsOrg {
		ctx.HTML(http.StatusOK, tplOrgSettingsActionsGeneral)
	} else {
		ctx.HTML(http.StatusOK, tplUserSettingsActionsGeneral)
	}
}

// UpdateGeneralSettings responses for actions general settings page
func UpdateGeneralSettings(ctx *context.Context) {
	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerError("getRunnersCtx", err)
		return
	}

	if !rCtx.IsOrg && !rCtx.IsUser {
		ctx.NotFound(nil)
		return
	}

	actionsCfg, err := actions_model.GetUserActionsConfig(ctx, rCtx.OwnerID)
	if err != nil {
		ctx.ServerError("GetUserActionsConfig", err)
		return
	}

	if ctx.FormBool("cross_repo_add_target") {
		targetRepoName := ctx.FormString("cross_repo_add_target_name")
		if targetRepoName != "" {
			targetRepo, err := repo_model.GetRepositoryByName(ctx, rCtx.OwnerID, targetRepoName)
			if err != nil {
				if repo_model.IsErrRepoNotExist(err) {
					ctx.JSONError("Repository doesn't exist")
					return
				}
				ctx.ServerError("GetRepositoryByName", err)
				return
			}
			if !slices.Contains(actionsCfg.AllowedCrossRepoIDs, targetRepo.ID) {
				actionsCfg.AllowedCrossRepoIDs = append(actionsCfg.AllowedCrossRepoIDs, targetRepo.ID)
			}
		}
	}

	if crossRepoRemoveTargetID := ctx.FormInt64("cross_repo_remove_target_id"); crossRepoRemoveTargetID != 0 {
		actionsCfg.AllowedCrossRepoIDs = util.SliceRemoveAll(actionsCfg.AllowedCrossRepoIDs, crossRepoRemoveTargetID)
	}

	// Update Cross-Repo Access Mode
	crossRepoMode := repo_model.ActionsCrossRepoMode(ctx.FormString("cross_repo_mode"))
	if crossRepoMode != "" {
		switch crossRepoMode {
		case repo_model.ActionsCrossRepoModeSelected:
		default:
			crossRepoMode = repo_model.ActionsCrossRepoModeNone
		}
		actionsCfg.CrossRepoMode = crossRepoMode
	}

	// Update Token Permission Mode
	tokenPermissionMode := repo_model.ActionsTokenPermissionMode(ctx.FormString("token_permission_mode"))
	if tokenPermissionMode != "" {
		switch tokenPermissionMode {
		case repo_model.ActionsTokenPermissionModeRestricted:
		default:
			tokenPermissionMode = repo_model.ActionsTokenPermissionModePermissive
		}
		actionsCfg.TokenPermissionMode = tokenPermissionMode
		enableMaxPermissions := ctx.FormBool("enable_max_permissions")
		// Update Maximum Permissions (radio buttons: none/read/write)
		if enableMaxPermissions {
			actionsCfg.MaxTokenPermissions = ParseMaxTokenPermissions(ctx)
		} else {
			actionsCfg.MaxTokenPermissions = nil
		}
	}

	if err := actions_model.SetUserActionsConfig(ctx, rCtx.OwnerID, actionsCfg); err != nil {
		ctx.ServerError("SetUserActionsConfig", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("settings.saved_successfully"))
	ctx.Redirect(ctx.Link)
}
