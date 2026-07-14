// Package setting provides web handlers for organization settings.
// Handlers for Actions permissions settings page. Modified by LAC | Ludwig investing
package setting

import (
	"code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/context"
)

// ActionsSettings renders the organization Actions permissions settings page.
// Added to support configurable permissions. Modified by LAC | Ludwig investing
func ActionsSettings(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("org.actions")
	ctx.Data["PageIsSettingsActions"] = true

	org := ctx.Org.Organization
	perms, err := actions.GetOrgActionsPermissions(ctx, org.ID)
	if err != nil {
		ctx.ServerError("GetOrgActionsPermissions", err)
		return
	}
	if perms != nil {
		permMap, err := perms.GetPermissionsMap()
		if err != nil {
			ctx.ServerError("GetPermissionsMap", err)
			return
		}
		ctx.Data["Permissions"] = permMap
	} else {
		ctx.Data["Permissions"] = getDefaultPermissionsMap()
	}
	ctx.HTML(200, "org/settings/actions")
}

// ActionsSettingsPost handles the form submission to update org actions permissions.
// Added to support configurable permissions. Modified by LAC | Ludwig investing
func ActionsSettingsPost(ctx *context.Context) {
	org := ctx.Org.Organization
	perms := make(map[actions.Scope]actions.Permission)
	for _, scope := range actions.AllScopes {
		val := ctx.Req.FormValue("permissions[" + string(scope) + "]")
		perms[scope] = actions.PermissionFromString(val)
	}
	err := actions.UpdateOrgActionsPermissions(ctx, org.ID, perms)
	if err != nil {
		ctx.ServerError("UpdateOrgActionsPermissions", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("org.settings.actions.updated"))
	ctx.Redirect(ctx.Org.OrgLink + "/settings/actions")
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
