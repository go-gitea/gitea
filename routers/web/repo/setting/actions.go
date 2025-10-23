// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"net/http"
	"strings"

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

	if ctx.Repo.Repository.IsPrivate {
		collaborativeOwnerIDs := actionsUnit.ActionsConfig().CollaborativeOwnerIDs
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
