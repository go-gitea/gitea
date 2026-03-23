// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/actions"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	shared_actions "code.gitea.io/gitea/routers/web/shared/actions"
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
	ctx.Data["TokenPermissionModePermissive"] = repo_model.ActionsTokenPermissionModePermissive
	ctx.Data["TokenPermissionModeRestricted"] = repo_model.ActionsTokenPermissionModeRestricted

	// Follow owner config (only for repos in orgs)
	ctx.Data["OverrideOwnerConfig"] = actionsCfg.OverrideOwnerConfig
	if actionsCfg.OverrideOwnerConfig {
		ctx.Data["MaxTokenPermissions"] = actionsCfg.GetMaxTokenPermissions()
		ctx.Data["TokenPermissionMode"] = actionsCfg.TokenPermissionMode
		ctx.Data["EnableMaxTokenPermissions"] = actionsCfg.MaxTokenPermissions != nil
	} else {
		ownerActionsConfig, err := actions.GetOwnerActionsConfig(ctx, ctx.Repo.Repository.OwnerID)
		if err != nil {
			ctx.ServerError("GetOwnerActionsConfig", err)
			return
		}
		ctx.Data["MaxTokenPermissions"] = ownerActionsConfig.GetMaxTokenPermissions()
		ctx.Data["TokenPermissionMode"] = ownerActionsConfig.TokenPermissionMode
		ctx.Data["EnableMaxTokenPermissions"] = ownerActionsConfig.MaxTokenPermissions != nil
	}

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
	if err := repo_model.UpdateRepoUnitConfig(ctx, actionsUnit); err != nil {
		ctx.ServerError("UpdateRepoUnitConfig", err)
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
	if err := repo_model.UpdateRepoUnitConfig(ctx, actionsUnit); err != nil {
		ctx.ServerError("UpdateRepoUnitConfig", err)
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

	// Update Override Owner Config (for repos in orgs)
	// If checked, it means we WANT to override (opt-out of following)
	actionsCfg.OverrideOwnerConfig = ctx.FormBool("override_owner_config")

	// Update permission mode (only if overriding owner config)
	shouldUpdate := actionsCfg.OverrideOwnerConfig

	if shouldUpdate {
		permissionMode, permissionModeValid := util.EnumValue(repo_model.ActionsTokenPermissionMode(ctx.FormString("token_permission_mode")))
		if !permissionModeValid {
			ctx.Flash.Error("Invalid token permission mode")
			ctx.Redirect(redirectURL)
			return
		}
		actionsCfg.TokenPermissionMode = permissionMode
	}

	// Update Maximum Permissions (radio buttons: none/read/write)
	enableMaxPermissions := ctx.FormBool("enable_max_permissions")
	if shouldUpdate {
		if enableMaxPermissions {
			actionsCfg.MaxTokenPermissions = shared_actions.ParseMaxTokenPermissions(ctx)
		} else {
			// If not enabled, ensure any sent permissions are ignored and set to nil
			actionsCfg.MaxTokenPermissions = nil
		}
	}

	if err := repo_model.UpdateRepoUnitConfig(ctx, actionsUnit); err != nil {
		ctx.ServerError("UpdateRepoUnitConfig", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
	ctx.Redirect(redirectURL)
}
