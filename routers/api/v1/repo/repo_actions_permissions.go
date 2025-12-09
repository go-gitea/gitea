// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/services/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
)

// GetActionsPermissions returns the Actions token permissions for a repository
func GetActionsPermissions(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/settings/actions/permissions repository repoGetActionsPermissions
	// ---
	// summary: Get repository Actions token permissions
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
	//     "$ref": "#/responses/ActionsPermissionsResponse"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// Check if user has admin access to this repo
	// NOTE: Only repo admins should be able to view/modify permission settings
	// This is important for security - we don't want regular contributors
	// to be able to grant themselves elevated permissions via Actions
	if !ctx.Repo.IsAdmin() {
		ctx.APIError(http.StatusForbidden, "You must be a repository admin to access this")
		return
	}

	perms, err := actions_model.GetRepoActionPermissions(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.APIError(http.StatusInternalServerError, err)
		return
	}

	// If no custom permissions are set, return the default (restricted mode)
	// This is intentional - we want a secure default that requires explicit opt-in
	// to more permissive settings. See: https://github.com/go-gitea/gitea/issues/24635
	if perms == nil {
		perms = &actions_model.ActionTokenPermission{
			RepoID:         ctx.Repo.Repository.ID,
			PermissionMode: actions_model.PermissionModeRestricted,
			// Default restricted permissions - only read contents and metadata
			ContentsRead: true,
			MetadataRead: true,
		}
	}

	ctx.JSON(http.StatusOK, convertToAPIPermissions(perms))
}

// UpdateActionsPermissions updates the Actions token permissions for a repository
func UpdateActionsPermissions(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/settings/actions/permissions repository repoUpdateActionsPermissions
	// ---
	// summary: Update repository Actions token permissions
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
	//     "$ref": "#/definitions/ActionsPermissions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActionsPermissionsResponse"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"

	if !ctx.Repo.IsAdmin() {
		ctx.APIError(http.StatusForbidden, "You must be a repository admin to modify this")
		return
	}

	form := web.GetForm(ctx).(*api.ActionsPermissions)

	// Validate permission mode
	if form.PermissionMode < 0 || form.PermissionMode > 2 {
		ctx.APIError(http.StatusUnprocessableEntity, "Permission mode must be 0 (restricted), 1 (permissive), or 2 (custom)")
		return
	}

	// TODO: Check if org-level permissions exist and validate against them
	// For now, we'll implement basic validation, but we should enhance this
	// to ensure repo settings don't exceed org caps. This is important for
	// multi-repository organizations where admins want centralized control.
	// See wolfogre's comment: https://github.com/go-gitea/gitea/pull/24554#issuecomment-1537040811

	perm := &actions_model.ActionTokenPermission{
		RepoID:            ctx.Repo.Repository.ID,
		PermissionMode:    actions_model.PermissionMode(form.PermissionMode),
		ActionsRead:       form.ActionsRead,
		ActionsWrite:      form.ActionsWrite,
		ContentsRead:      form.ContentsRead,
		ContentsWrite:     form.ContentsWrite,
		IssuesRead:        form.IssuesRead,
		IssuesWrite:       form.IssuesWrite,
		PackagesRead:      form.PackagesRead,
		PackagesWrite:     form.PackagesWrite,
		PullRequestsRead:  form.PullRequestsRead,
		PullRequestsWrite: form.PullRequestsWrite,
		MetadataRead:      true, // Always true - needed for basic operations
	}

	if err := actions_model.CreateOrUpdateRepoPermissions(ctx, perm); err != nil {
		ctx.APIError(http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, convertToAPIPermissions(perm))
}

// ResetActionsPermissions resets permissions to default (restricted mode)
func ResetActionsPermissions(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/settings/actions/permissions repository repoResetActionsPermissions
	// ---
	// summary: Reset repository Actions permissions to default
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
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	if !ctx.Repo.IsAdmin() {
		ctx.APIError(http.StatusForbidden, "You must be a repository admin")
		return
	}

	// Create default restricted permissions
	// This is a "safe reset" - puts the repo back to secure defaults
	defaultPerm := &actions_model.ActionTokenPermission{
		RepoID:         ctx.Repo.Repository.ID,
		PermissionMode: actions_model.PermissionModeRestricted,
		ContentsRead:   true,
		MetadataRead:   true,
	}

	if err := actions_model.CreateOrUpdateRepoPermissions(ctx, defaultPerm); err != nil {
		ctx.APIError(http.StatusInternalServerError, err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// convertToAPIPermissions converts model to API response format
// This helper keeps our internal model separate from the API contract
func convertToAPIPermissions(perm *actions_model.ActionTokenPermission) *api.ActionsPermissions {
	return &api.ActionsPermissions{
		PermissionMode:    int(perm.PermissionMode),
		ActionsRead:       perm.ActionsRead,
		ActionsWrite:      perm.ActionsWrite,
		ContentsRead:      perm.ContentsRead,
		ContentsWrite:     perm.ContentsWrite,
		IssuesRead:        perm.IssuesRead,
		IssuesWrite:       perm.IssuesWrite,
		PackagesRead:      perm.PackagesRead,
		PackagesWrite:     perm.PackagesWrite,
		PullRequestsRead:  perm.PullRequestsRead,
		PullRequestsWrite: perm.PullRequestsWrite,
		MetadataRead:      perm.MetadataRead,
	}
}

