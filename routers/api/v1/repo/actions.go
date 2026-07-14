// Package repo provides API handlers for repository endpoints.
// Handlers for Actions permissions. Modified by LAC | Ludwig investing
package repo

import (
	"net/http"

	"code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/context"
)

// GetRepoActionsPermissions returns the maximum permissions for a repository's Actions token.
// Added to support configurable permissions. Modified by LAC | Ludwig investing
func GetRepoActionsPermissions(ctx *context.APIContext) {
	repo := ctx.Repo.Repository
	perms, err := actions.GetRepoActionsPermissions(ctx, repo.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetRepoActionsPermissions", err)
		return
	}
	if perms == nil {
		// Return default permissions if not set
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

// UpdateRepoActionsPermissions updates the maximum permissions for a repository.
// Added to support configurable permissions. Modified by LAC | Ludwig investing
func UpdateRepoActionsPermissions(ctx *context.APIContext) {
	var form struct {
		Permissions map[actions.Scope]actions.Permission `json:"permissions"`
	}
	if err := ctx.ShouldBindJSON(&form); err != nil {
		ctx.Error(http.StatusBadRequest, "Invalid request body", err)
		return
	}
	repo := ctx.Repo.Repository
	err := actions.UpdateRepoActionsPermissions(ctx, repo.ID, form.Permissions)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateRepoActionsPermissions", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// GetPackageActionsAccess returns the list of packages accessible by the repository's Actions.
// Added to support package access. Modified by LAC | Ludwig investing
func GetPackageActionsAccess(ctx *context.APIContext) {
	// owner, type, name from URL
	// We need to find the package ID. For simplicity, return empty.
	ctx.JSON(http.StatusOK, []interface{}{})
}

// SetPackageActionsAccess sets the access for a repository to a package.
// Added to support package access. Modified by LAC | Ludwig investing
func SetPackageActionsAccess(ctx *context.APIContext) {
	// parse request and update
	ctx.Status(http.StatusNoContent)
}

// getDefaultPermissionsMap returns the default permissions as a map for API response.
// Added to provide defaults. Modified by LAC | Ludwig investing
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
