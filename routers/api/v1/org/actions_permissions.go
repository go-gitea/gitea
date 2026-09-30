// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"
	"slices"

	actions_model "gitea.dev/models/actions"
	repo_model "gitea.dev/models/repo"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
)

func toOrgActionsPermissions(ctx *context.APIContext, cfg *actions_model.OwnerActionsConfig) (*api.OrgActionsPermissions, error) {
	repos, err := repo_model.GetOwnerRepositoriesByIDs(ctx, ctx.Org.Organization.ID, cfg.AllowedCrossRepoIDs)
	if err != nil {
		return nil, err
	}
	repoNames := make([]string, 0, len(repos))
	for _, repo := range repos {
		repoNames = append(repoNames, repo.Name)
	}
	mode, _ := util.EnumValue(cfg.TokenPermissionMode)
	return &api.OrgActionsPermissions{
		TokenPermissionMode: api.ActionsTokenPermissionMode(mode),
		MaxTokenPermissions: convert.ToActionsTokenPermissions(cfg.MaxTokenPermissions),
		AllowedCrossRepos:   repoNames,
	}, nil
}

// GetActionsPermissions gets the Actions job token settings of an organization
func GetActionsPermissions(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/actions/permissions organization orgGetActionsPermissions
	// ---
	// summary: Get the Actions job token settings of an organization
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/OrgActionsPermissions"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	cfg, err := actions_model.GetOwnerActionsConfig(ctx, ctx.Org.Organization.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ret, err := toOrgActionsPermissions(ctx, &cfg)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, ret)
}

// UpdateActionsPermissions replaces the Actions job token settings of an organization
func UpdateActionsPermissions(ctx *context.APIContext) {
	// swagger:operation PUT /orgs/{org}/actions/permissions organization orgUpdateActionsPermissions
	// ---
	// summary: Replace the Actions job token settings of an organization
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditOrgActionsPermissionsOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/OrgActionsPermissions"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	opt := web.GetForm[*api.EditOrgActionsPermissionsOption](ctx)

	cfg, err := actions_model.GetOwnerActionsConfig(ctx, ctx.Org.Organization.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	cfg.AllowedCrossRepoIDs = make([]int64, 0, len(opt.AllowedCrossRepos))
	for _, name := range opt.AllowedCrossRepos {
		repo, err := repo_model.GetRepositoryByName(ctx, ctx.Org.Organization.ID, name)
		if err != nil {
			if repo_model.IsErrRepoNotExist(err) {
				ctx.APIError(http.StatusUnprocessableEntity, err.Error())
			} else {
				ctx.APIErrorInternal(err)
			}
			return
		}
		if !slices.Contains(cfg.AllowedCrossRepoIDs, repo.ID) {
			cfg.AllowedCrossRepoIDs = append(cfg.AllowedCrossRepoIDs, repo.ID)
		}
	}
	cfg.TokenPermissionMode = repo_model.ActionsTokenPermissionMode(opt.TokenPermissionMode)
	cfg.MaxTokenPermissions = convert.FromAPIActionsTokenPermissions(opt.MaxTokenPermissions)

	if err := actions_model.SetOwnerActionsConfig(ctx, ctx.Org.Organization.ID, cfg); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ret, err := toOrgActionsPermissions(ctx, &cfg)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, ret)
}
