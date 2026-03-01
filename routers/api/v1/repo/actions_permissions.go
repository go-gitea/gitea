// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/convert"
)

// GetActionsTokenPermissions gets the Actions token permissions for a repository
func GetActionsTokenPermissions(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/permissions repository repoGetActionsTokenPermissions
	// ---
	// summary: Get Actions token permissions for a repository
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
	//     "$ref": "#/responses/ActionsTokenPermissions"
	//   "404":
	//     "$ref": "#/responses/notFound"

	perms, err := actions_model.GetActionTokenPermissions(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetActionTokenPermissions", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToActionsTokenPermissions(perms))
}

// SetActionsTokenPermissions sets the Actions token permissions for a repository
func SetActionsTokenPermissions(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/actions/permissions repository repoSetActionsTokenPermissions
	// ---
	// summary: Set Actions token permissions for a repository
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
	//     "$ref": "#/definitions/ActionsTokenPermissions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActionsTokenPermissions"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := api.ActionsTokenPermissions{}
	if err := ctx.Bind(&form); err != nil {
		ctx.Error(http.StatusUnprocessableEntity, "Bind", err)
		return
	}

	perms := &actions_model.ActionTokenPermissions{
		RepoID:                 ctx.Repo.Repository.ID,
		DefaultPermissions:     form.DefaultPermissions,
		ContentsPermission:     form.ContentsPermission,
		IssuesPermission:       form.IssuesPermission,
		PullRequestsPermission: form.PullRequestsPermission,
		PackagesPermission:     form.PackagesPermission,
		MetadataPermission:     form.MetadataPermission,
		ActionsPermission:      form.ActionsPermission,
		OrganizationPermission: form.OrganizationPermission,
		NotificationPermission: form.NotificationPermission,
	}

	// Validate permission values
	validPerms := map[string]bool{"read": true, "write": true, "none": true, "": true}
	if !validPerms[perms.DefaultPermissions] {
		ctx.Error(http.StatusUnprocessableEntity, "InvalidDefaultPermissions", "default_permissions must be 'read', 'write', or 'none'")
		return
	}

	if err := actions_model.SetActionTokenPermissions(ctx, perms); err != nil {
		ctx.Error(http.StatusInternalServerError, "SetActionTokenPermissions", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToActionsTokenPermissions(perms))
}
