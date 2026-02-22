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

	ctx.Data["TokenPermissionMode"] = actionsCfg.GetTokenPermissionMode()
	ctx.Data["TokenPermissionModePermissive"] = repo_model.ActionsTokenPermissionModePermissive
	ctx.Data["TokenPermissionModeRestricted"] = repo_model.ActionsTokenPermissionModeRestricted
	ctx.Data["TokenPermissionModeCustom"] = repo_model.ActionsTokenPermissionModeCustom
	ctx.Data["MaxTokenPermissions"] = actionsCfg.GetMaxTokenPermissions()
	ctx.Data["EnableMaxTokenPermissions"] = actionsCfg.MaxTokenPermissions != nil

	ctx.Data["CrossRepoMode"] = actionsCfg.CrossRepoMode
	ctx.Data["ActionsCrossRepoModeNone"] = repo_model.ActionsCrossRepoModeNone
	ctx.Data["ActionsCrossRepoModeAll"] = repo_model.ActionsCrossRepoModeAll
	ctx.Data["ActionsCrossRepoModeSelected"] = repo_model.ActionsCrossRepoModeSelected

	// Load Allowed Repositories
	var allowedRepos []*repo_model.Repository
	if len(actionsCfg.AllowedCrossRepoIDs) > 0 {
		allowedRepos, err = repo_model.GetRepositoriesByIDs(ctx, actionsCfg.AllowedCrossRepoIDs)
		if err != nil {
			ctx.ServerError("GetRepositoriesByIDs", err)
			return
		}
	}
	ctx.Data["AllowedRepos"] = allowedRepos
	ctx.Data["OwnerID"] = rCtx.OwnerID

	generalLink := rCtx.RedirectLink + "../general"
	ctx.Data["Link"] = generalLink

	if rCtx.IsOrg {
		ctx.HTML(http.StatusOK, tplOrgSettingsActionsGeneral)
	} else {
		ctx.HTML(http.StatusOK, tplUserSettingsActionsGeneral)
	}
}

// UpdateTokenPermissions responses for actions general settings page
func UpdateTokenPermissions(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.actions")

	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerError("getRunnersCtx", err)
		return
	}

	if rCtx.IsOrg {
		ctx.Data["PageIsOrgSettings"] = true
		ctx.Data["PageIsOrgSettingsActions"] = true
	} else if rCtx.IsUser {
		ctx.Data["PageIsUserSettings"] = true
		ctx.Data["PageIsUserSettingsActions"] = true
	} else {
		ctx.NotFound(nil)
		return
	}

	actionsCfg, err := actions_model.GetUserActionsConfig(ctx, rCtx.OwnerID)
	if err != nil {
		ctx.ServerError("GetUserActionsConfig", err)
		return
	}

	// Update Token Permission Mode
	permissionMode := repo_model.ActionsTokenPermissionMode(ctx.FormString("token_permission_mode"))
	if permissionMode == repo_model.ActionsTokenPermissionModeRestricted ||
		permissionMode == repo_model.ActionsTokenPermissionModePermissive ||
		permissionMode == repo_model.ActionsTokenPermissionModeCustom {
		actionsCfg.TokenPermissionMode = permissionMode
	}

	enableMaxPermissions := ctx.FormBool("enable_max_permissions")
	// Update Maximum Permissions (radio buttons: none/read/write)
	if enableMaxPermissions {
		actionsCfg.MaxTokenPermissions = ParseMaxTokenPermissions(ctx)
	} else {
		actionsCfg.MaxTokenPermissions = nil
	}

	// Update Cross-Repo Access Mode
	crossRepoMode := ctx.FormString("cross_repo_mode")
	switch crossRepoMode {
	case "none":
		actionsCfg.CrossRepoMode = repo_model.ActionsCrossRepoModeNone
		actionsCfg.AllowedCrossRepoIDs = nil
	case "all":
		actionsCfg.CrossRepoMode = repo_model.ActionsCrossRepoModeAll
		actionsCfg.AllowedCrossRepoIDs = nil
	case "selected":
		actionsCfg.CrossRepoMode = repo_model.ActionsCrossRepoModeSelected
		// Keep existing AllowedCrossRepoIDs, will be updated by separate API
	default:
		// Default to none if invalid
		actionsCfg.CrossRepoMode = repo_model.ActionsCrossRepoModeNone
	}

	if err := actions_model.SetUserActionsConfig(ctx, rCtx.OwnerID, actionsCfg); err != nil {
		ctx.ServerError("SetUserActionsConfig", err)
		return
	}

	generalLink := rCtx.RedirectLink + "../general"

	if rCtx.IsOrg {
		ctx.Flash.Success(ctx.Tr("org.settings.update_setting_success"))
	} else {
		ctx.Flash.Success(ctx.Tr("settings.update_settings_success"))
	}
	ctx.Redirect(generalLink)
}

// AllowedReposAdd adds a repository to the allowed list for cross-repo access
func AllowedReposAdd(ctx *context.Context) {
	repoName := ctx.FormString("repo_name")

	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerError("getRunnersCtx", err)
		return
	}

	// Actions config path is up one level from runners
	actionsLink := rCtx.RedirectLink + ".."
	generalLink := rCtx.RedirectLink + "../general"

	if repoName == "" {
		ctx.Redirect(actionsLink)
		return
	}

	repo, err := repo_model.GetRepositoryByName(ctx, rCtx.OwnerID, repoName)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			ctx.Flash.Error(ctx.Tr("repo.not_exist"))
			ctx.Redirect(actionsLink)
			return
		}
		ctx.ServerError("GetRepositoryByName", err)
		return
	}

	actionsCfg, err := actions_model.GetUserActionsConfig(ctx, rCtx.OwnerID)
	if err != nil {
		ctx.ServerError("GetUserActionsConfig", err)
		return
	}

	// Check if already exists
	if slices.Contains(actionsCfg.AllowedCrossRepoIDs, repo.ID) {
		ctx.Redirect(actionsLink)
		return
	}

	actionsCfg.AllowedCrossRepoIDs = append(actionsCfg.AllowedCrossRepoIDs, repo.ID)

	if err := actions_model.SetUserActionsConfig(ctx, rCtx.OwnerID, actionsCfg); err != nil {
		ctx.ServerError("SetUserActionsConfig", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
	ctx.Redirect(generalLink)
}

// AllowedReposRemove removes a repository from the allowed list
func AllowedReposRemove(ctx *context.Context) {
	repoID := ctx.FormInt64("repo_id")

	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerError("getRunnersCtx", err)
		return
	}

	actionsLink := rCtx.RedirectLink + ".."
	generalLink := rCtx.RedirectLink + "../general"

	if repoID == 0 {
		ctx.Redirect(actionsLink)
		return
	}

	actionsCfg, err := actions_model.GetUserActionsConfig(ctx, rCtx.OwnerID)
	if err != nil {
		ctx.ServerError("GetUserActionsConfig", err)
		return
	}

	// Filter out the ID
	newIDs := make([]int64, 0, len(actionsCfg.AllowedCrossRepoIDs))
	for _, id := range actionsCfg.AllowedCrossRepoIDs {
		if id != repoID {
			newIDs = append(newIDs, id)
		}
	}
	actionsCfg.AllowedCrossRepoIDs = newIDs

	if err := actions_model.SetUserActionsConfig(ctx, rCtx.OwnerID, actionsCfg); err != nil {
		ctx.ServerError("SetUserActionsConfig", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
	ctx.Redirect(generalLink)
}
