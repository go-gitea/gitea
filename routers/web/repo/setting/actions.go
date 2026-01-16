// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
	repo_service "code.gitea.io/gitea/services/repository"
)

const tplRepoActionsGeneralSettings templates.TplName = "repo/settings/actions"

func ActionsGeneralSettings(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.general")
	ctx.Data["PageType"] = "general"
	ctx.Data["PageIsActionsSettingsGeneral"] = true

	actionsUnit, err := ctx.Repo.Repository.GetUnit(ctx, unit_model.TypeActions)
	if err != nil && !repo_model.IsErrUnitTypeNotExist(err) {
		ctx.ServerError("GetUnit", err)
		return
	}
	if actionsUnit == nil { // no actions unit
		ctx.HTML(http.StatusOK, tplRepoActionsGeneralSettings)
		return
	}

	actionsCfg := actionsUnit.ActionsConfig()

	// Token permission settings
	ctx.Data["TokenPermissionMode"] = actionsCfg.GetTokenPermissionMode()
	ctx.Data["TokenPermissionModePermissive"] = repo_model.ActionsTokenPermissionModePermissive
	ctx.Data["TokenPermissionModeRestricted"] = repo_model.ActionsTokenPermissionModeRestricted
	ctx.Data["TokenPermissionModeCustom"] = repo_model.ActionsTokenPermissionModeCustom
	ctx.Data["MaxTokenPermissions"] = actionsCfg.GetMaxTokenPermissions()

	// Follow org config (only for repos in orgs)
	ctx.Data["IsInOrg"] = ctx.Repo.Repository.Owner.IsOrganization()
	ctx.Data["OverrideOrgConfig"] = actionsCfg.OverrideOrgConfig

	if ctx.Repo.Repository.IsPrivate {
		collaborativeOwnerIDs := actionsCfg.CollaborativeOwnerIDs
		collaborativeOwners, err := user_model.GetUsersByIDs(ctx, collaborativeOwnerIDs)
		if err != nil {
			ctx.ServerError("GetUsersByIDs", err)
			return
		}
		ctx.Data["CollaborativeOwners"] = collaborativeOwners
	}

	ctx.HTML(http.StatusOK, tplRepoActionsGeneralSettings)
}

func ActionsUnitPost(ctx *context.Context) {
	redirectURL := ctx.Repo.RepoLink + "/settings/actions/general"
	enableActionsUnit := ctx.FormBool("enable_actions")
	repo := ctx.Repo.Repository

	var err error
	if enableActionsUnit && !unit_model.TypeActions.UnitGlobalDisabled() {
		err = repo_service.UpdateRepositoryUnits(ctx, repo, []repo_model.RepoUnit{newRepoUnit(repo, unit_model.TypeActions, nil)}, nil)
	} else if !unit_model.TypeActions.UnitGlobalDisabled() {
		err = repo_service.UpdateRepositoryUnits(ctx, repo, nil, []unit_model.Type{unit_model.TypeActions})
	}

	if err != nil {
		ctx.ServerError("UpdateRepositoryUnits", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
	ctx.Redirect(redirectURL)
}

func AddCollaborativeOwner(ctx *context.Context) {
	name := strings.ToLower(ctx.FormString("collaborative_owner"))

	ownerID, err := user_model.GetUserOrOrgIDByName(ctx, name)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Flash.Error(ctx.Tr("form.user_not_exist"))
			ctx.JSONErrorNotFound()
		} else {
			ctx.ServerError("GetUserOrOrgIDByName", err)
		}
		return
	}

	actionsUnit, err := ctx.Repo.Repository.GetUnit(ctx, unit_model.TypeActions)
	if err != nil {
		ctx.ServerError("GetUnit", err)
		return
	}
	actionsCfg := actionsUnit.ActionsConfig()
	actionsCfg.AddCollaborativeOwner(ownerID)
	if err := repo_model.UpdateRepoUnit(ctx, actionsUnit); err != nil {
		ctx.ServerError("UpdateRepoUnit", err)
		return
	}

	ctx.JSONOK()
}

func DeleteCollaborativeOwner(ctx *context.Context) {
	ownerID := ctx.FormInt64("id")

	actionsUnit, err := ctx.Repo.Repository.GetUnit(ctx, unit_model.TypeActions)
	if err != nil {
		ctx.ServerError("GetUnit", err)
		return
	}
	actionsCfg := actionsUnit.ActionsConfig()
	if !actionsCfg.IsCollaborativeOwner(ownerID) {
		ctx.Flash.Error(ctx.Tr("actions.general.collaborative_owner_not_exist"))
		ctx.JSONErrorNotFound()
		return
	}
	actionsCfg.RemoveCollaborativeOwner(ownerID)
	if err := repo_model.UpdateRepoUnit(ctx, actionsUnit); err != nil {
		ctx.ServerError("UpdateRepoUnit", err)
		return
	}

	ctx.JSONOK()
}

// UpdateTokenPermissions updates the token permission settings for the repository
func UpdateTokenPermissions(ctx *context.Context) {
	redirectURL := ctx.Repo.RepoLink + "/settings/actions/general"

	actionsUnit, err := ctx.Repo.Repository.GetUnit(ctx, unit_model.TypeActions)
	if err != nil {
		ctx.ServerError("GetUnit", err)
		return
	}

	actionsCfg := actionsUnit.ActionsConfig()

	// Update Override Org Config (for repos in orgs)
	// If checked, it means we WANT to override (opt-out of following)
	actionsCfg.OverrideOrgConfig = ctx.FormBool("override_org_config")

	// Update permission mode (only if overriding org config)
	if actionsCfg.OverrideOrgConfig {
		permissionMode := repo_model.ActionsTokenPermissionMode(ctx.FormString("token_permission_mode"))
		if permissionMode == repo_model.ActionsTokenPermissionModeRestricted ||
			permissionMode == repo_model.ActionsTokenPermissionModePermissive ||
			permissionMode == repo_model.ActionsTokenPermissionModeCustom {
			actionsCfg.TokenPermissionMode = permissionMode
		} else {
			ctx.Flash.Error("Invalid token permission mode")
			ctx.Redirect(redirectURL)
			return
		}
	}

	// Update Maximum Permissions (radio buttons: none/read/write)
	if actionsCfg.OverrideOrgConfig && actionsCfg.TokenPermissionMode == repo_model.ActionsTokenPermissionModeCustom {
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
			Code:         parseMaxPerm("code"),
			Issues:       parseMaxPerm("issues"),
			Packages:     parseMaxPerm("packages"),
			PullRequests: parseMaxPerm("pull_requests"),
			Wiki:         parseMaxPerm("wiki"),
			Actions:      parseMaxPerm("actions"),
			Releases:     parseMaxPerm("releases"),
			Projects:     parseMaxPerm("projects"),
		}
	} else {
		actionsCfg.MaxTokenPermissions = nil
	}

	if err := repo_model.UpdateRepoUnit(ctx, actionsUnit); err != nil {
		ctx.ServerError("UpdateRepoUnit", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
	ctx.Redirect(redirectURL)
}
