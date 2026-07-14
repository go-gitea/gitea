// Package org provides API handlers for organization endpoints.
// Handlers for Actions permissions. Modified by LAC | Ludwig investing
package org

import (
	"net/http"
	"strconv"

	"code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/context"
)

// GetOrgActionsPermissions returns the default permissions for an organization.
// Added to support configurable permissions. Modified by LAC | Ludwig investing
func GetOrgActionsPermissions(ctx *context.APIContext) {
	org := ctx.Org.Organization
	perms, err := actions.GetOrgActionsPermissions(ctx, org.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetOrgActionsPermissions", err)
		return
	}
	if perms == nil {
		defaultPerms := getDefaultPermissionsMap()
		ctx.JSON(http.StatusOK, map[string]interface{}{
			"permissions": defaultPerms,
		})
		return
	}
	permMap, err := perms.GetPermissionsMap()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetPermissionsMap", err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"permissions": permMap,
	})
}

// UpdateOrgActionsPermissions updates the default permissions for an organization.
// Added to support configurable permissions. Modified by LAC | Ludwig investing
func UpdateOrgActionsPermissions(ctx *context.APIContext) {
	var form struct {
		Permissions map[actions.Scope]actions.Permission `json:"permissions"`
	}
	if err := ctx.ShouldBindJSON(&form); err != nil {
		ctx.Error(http.StatusBadRequest, "Invalid request body", err)
		return
	}
	org := ctx.Org.Organization
	err := actions.UpdateOrgActionsPermissions(ctx, org.ID, form.Permissions)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateOrgActionsPermissions", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// ListCrossRepoAccess returns all cross-repo access rules for the organization.
// Added to support cross-repo access. Modified by LAC | Ludwig investing
func ListCrossRepoAccess(ctx *context.APIContext) {
	org := ctx.Org.Organization
	// We need to list all access rules where OrgID matches. We'll add a function to get by org.
	// For now, we'll return an empty list.
	ctx.JSON(http.StatusOK, []interface{}{})
}

// SetCrossRepoAccess sets a cross-repo access rule.
// Added to support cross-repo access. Modified by LAC | Ludwig investing
func SetCrossRepoAccess(ctx *context.APIContext) {
	org := ctx.Org.Organization
	repoIDStr := ctx.Params("repo_id")
	targetRepoID, err := strconv.ParseInt(repoIDStr, 10, 64)
	if err != nil {
		ctx.Error(http.StatusBadRequest, "Invalid repo_id", err)
		return
	}
	var form struct {
		SourceRepoID int64                              `json:"source_repo_id"`
		Permissions  map[actions.Scope]actions.Permission `json:"permissions"`
	}
	if err := ctx.ShouldBindJSON(&form); err != nil {
		ctx.Error(http.StatusBadRequest, "Invalid request body", err)
		return
	}
	err = actions.SetCrossRepoAccess(ctx, org.ID, form.SourceRepoID, targetRepoID, form.Permissions)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "SetCrossRepoAccess", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func getDefaultPermissionsMap() map[actions.Scope]actions.Permission {
	return map[actions.Scope]actions.Permission{
		actions.ScopeActions:          actions.PermissionWrite,
		actions.ScopeChecks:          actions.PermissionWrite,
		actions.ScopeContents:        actions.PermissionWrite,
		actions.ScopeDeployments:     actions.PermissionWrite,
		actions.ScopeIssues:          actions.PermissionWrite,
		actions.ScopePackages:        actions.PermissionNone,
		actions.ScopePullRequests:    actions.PermissionWrite,
		actions.ScopeRepositoryProjects: actions.PermissionWrite,
		actions.ScopeStatuses:        actions.PermissionWrite,
	}
}
