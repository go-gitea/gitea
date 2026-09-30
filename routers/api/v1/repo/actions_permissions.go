// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	actions_model "gitea.dev/models/actions"
	repo_model "gitea.dev/models/repo"
	unit_model "gitea.dev/models/unit"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
)

func getActionsUnit(ctx *context.APIContext) *repo_model.RepoUnit {
	actionsUnit, err := ctx.Repo.Repository.GetUnit(ctx, unit_model.TypeActions)
	if repo_model.IsErrUnitTypeNotExist(err) {
		ctx.APIErrorNotFound("actions unit is not enabled")
		return nil
	} else if err != nil {
		ctx.APIErrorInternal(err)
		return nil
	}
	return actionsUnit
}

func respondRepoActionsPermissions(ctx *context.APIContext, cfg *repo_model.ActionsConfig) {
	mode, maxPerms := cfg.TokenPermissionMode, cfg.MaxTokenPermissions
	if !cfg.OverrideOwnerConfig {
		ownerCfg, err := actions_model.GetOwnerActionsConfig(ctx, ctx.Repo.Repository.OwnerID)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		mode, maxPerms = ownerCfg.TokenPermissionMode, ownerCfg.MaxTokenPermissions
	}
	mode, _ = util.EnumValue(mode)
	ctx.JSON(http.StatusOK, &api.RepoActionsPermissions{
		OverrideOwnerConfig: cfg.OverrideOwnerConfig,
		TokenPermissionMode: api.ActionsTokenPermissionMode(mode),
		MaxTokenPermissions: convert.ToActionsTokenPermissions(maxPerms),
	})
}

// GetActionsPermissions gets the Actions job token settings of a repository
func GetActionsPermissions(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/permissions repository repoGetActionsPermissions
	// ---
	// summary: Get the Actions job token settings of a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepoActionsPermissions"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	actionsUnit := getActionsUnit(ctx)
	if actionsUnit == nil {
		return
	}
	respondRepoActionsPermissions(ctx, actionsUnit.ActionsConfig())
}

// UpdateActionsPermissions replaces the Actions job token settings of a repository
func UpdateActionsPermissions(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/actions/permissions repository repoUpdateActionsPermissions
	// ---
	// summary: Replace the Actions job token settings of a repository
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditRepoActionsPermissionsOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepoActionsPermissions"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	opt := web.GetForm[*api.EditRepoActionsPermissionsOption](ctx)

	actionsUnit := getActionsUnit(ctx)
	if actionsUnit == nil {
		return
	}
	cfg := actionsUnit.ActionsConfig()
	cfg.OverrideOwnerConfig = opt.OverrideOwnerConfig
	if opt.OverrideOwnerConfig {
		if opt.TokenPermissionMode == "" {
			ctx.APIError(http.StatusUnprocessableEntity, "token_permission_mode is required when overriding the owner config")
			return
		}
		cfg.TokenPermissionMode = repo_model.ActionsTokenPermissionMode(opt.TokenPermissionMode)
		cfg.MaxTokenPermissions = convert.FromAPIActionsTokenPermissions(opt.MaxTokenPermissions)
	}

	if err := repo_model.UpdateRepoUnitConfig(ctx, actionsUnit); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	respondRepoActionsPermissions(ctx, cfg)
}
