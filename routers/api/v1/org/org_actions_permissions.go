// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"

	actions_model "code.gitea.io/gitea/models/actions"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
)

// GetActionsPermissions returns the Actions token permissions for an organization
func GetActionsPermissions(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/settings/actions/permissions organization orgGetActionsPermissions
	// ---
	// summary: Get organization Actions token permissions
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
	//     "$ref": "#/responses/OrgActionsPermissionsResponse"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// Organization settings are more sensitive than repo settings because they
	// affect ALL repositories in the org. We should be extra careful here.
	// Only org owners should be able to modify these settings.
	// This is enforced by the reqOrgOwnership middleware.

	perms, err := actions_model.GetOrgActionPermissions(ctx, ctx.Org.Organization.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	// Return default if no custom config exists
	// Organizations default to restricted mode for maximum security
	// Individual repos can be given more permissions if needed
	if perms == nil {
		perms = &actions_model.ActionOrgPermission{
			OrgID:             ctx.Org.Organization.ID,
			PermissionMode:    actions_model.PermissionModeRestricted,
			AllowRepoOverride: true, // Allow repos to configure their own settings
			ContentsRead:      true,
			MetadataRead:      true,
		}
	}

	ctx.JSON(http.StatusOK, convertToAPIOrgPermissions(perms))
}

// UpdateActionsPermissions updates the Actions token permissions for an organization
func UpdateActionsPermissions(ctx *context.APIContext) {
	// swagger:operation PUT /orgs/{org}/settings/actions/permissions organization orgUpdateActionsPermissions
	// ---
	// summary: Update organization Actions token permissions
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
	//     "$ref": "#/definitions/OrgActionsPermissions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/OrgActionsPermissionsResponse"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	// Organization settings are more sensitive than repo settings because they
	// affect ALL repositories in the org. We should be extra careful here.
	// Only org owners should be able to modify these settings.
	// This is enforced by the reqOrgOwnership middleware.

	form := web.GetForm(ctx).(*api.OrgActionsPermissions)

	// Validate permission mode
	if form.PermissionMode < 0 || form.PermissionMode > 2 {
		ctx.APIError(http.StatusUnprocessableEntity, "Permission mode must be 0 (restricted), 1 (permissive), or 2 (custom)")
		return
	}

	// Important security consideration:
	// If AllowRepoOverride is false, ALL repos in this org MUST use org settings.
	// This is useful for security-conscious organizations that want centralized control.
	// However, it's a big change, so we should log this action for audit purposes.
	// TODO: Add audit logging when this feature is used

	perm := &actions_model.ActionOrgPermission{
		OrgID:             ctx.Org.Organization.ID,
		PermissionMode:    actions_model.PermissionMode(form.PermissionMode),
		AllowRepoOverride: form.AllowRepoOverride,
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
		MetadataRead:      true, // Always true
	}

	if err := actions_model.CreateOrUpdateOrgPermissions(ctx, perm); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	// If AllowRepoOverride is false, we might want to update all repo permissions
	// to match org settings. But that's a big operation, so let's do it lazily
	// when permissions are actually checked, rather than updating all repos here.
	// This is more performant and avoids potential race conditions.

	ctx.JSON(http.StatusOK, convertToAPIOrgPermissions(perm))
}

// ListCrossRepoAccess lists all cross-repository access rules for an organization
func ListCrossRepoAccess(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/settings/actions/cross-repo-access organization orgListCrossRepoAccess
	// ---
	// summary: List cross-repository access rules
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
	//     "$ref": "#/responses/CrossRepoAccessList"

	// This is a critical security feature - cross-repo access allows one repo's
	// Actions to access another repo's code/resources. We need to be very careful
	// about how we implement this. See the discussion:
	// https://github.com/go-gitea/gitea/issues/24635
	// Permission check handled by reqOrgOwnership middleware

	rules, err := actions_model.ListCrossRepoAccessRules(ctx, ctx.Org.Organization.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiRules := make([]*api.CrossRepoAccessRule, len(rules))
	for i, rule := range rules {
		apiRules[i] = convertToCrossRepoAccessRule(rule)
	}

	ctx.JSON(http.StatusOK, apiRules)
}

// AddCrossRepoAccess adds a new cross-repository access rule
func AddCrossRepoAccess(ctx *context.APIContext) {
	// swagger:operation POST /orgs/{org}/settings/actions/cross-repo-access organization orgAddCrossRepoAccess
	// ---
	// summary: Add cross-repository access rule
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
	//     "$ref": "#/definitions/CrossRepoAccessRule"
	// responses:
	//   "201":
	//     "$ref": "#/responses/CrossRepoAccessRule"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	// Permission check handled by reqOrgOwnership middleware

	form := web.GetForm(ctx).(*api.CrossRepoAccessRule)

	// Validation: source and target repos must both belong to this org
	// We don't want to allow cross-organization access - that would be a
	// security nightmare and makes audit trails very complex.
	// TODO: Verify both repos belong to this org

	// Validation: Access level must be valid (0=none, 1=read, 2=write)
	if form.AccessLevel < 0 || form.AccessLevel > 2 {
		ctx.APIError(http.StatusUnprocessableEntity, "Access level must be 0 (none), 1 (read), or 2 (write)")
		return
	}

	rule := &actions_model.ActionCrossRepoAccess{
		OrgID:        ctx.Org.Organization.ID,
		SourceRepoID: form.SourceRepoID,
		TargetRepoID: form.TargetRepoID,
		AccessLevel:  form.AccessLevel,
	}

	if err := actions_model.CreateCrossRepoAccess(ctx, rule); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusCreated, convertToCrossRepoAccessRule(rule))
}

// DeleteCrossRepoAccess removes a cross-repository access rule
func DeleteCrossRepoAccess(ctx *context.APIContext) {
	// swagger:operation DELETE /orgs/{org}/settings/actions/cross-repo-access/{id} organization orgDeleteCrossRepoAccess
	// ---
	// summary: Delete cross-repository access rule
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: ID of the rule to delete
	//   type: integer
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	// Permission check handled by reqOrgOwnership middleware
	ruleID := ctx.PathParamInt64("id")

	// Security check: Verify the rule belongs to this org before deleting
	// We don't want one org to be able to delete another org's rules
	rule, err := actions_model.GetCrossRepoAccessByID(ctx, ruleID)
	if err != nil {
		ctx.APIError(http.StatusNotFound, "Cross-repo access rule not found")
		return
	}

	if rule.OrgID != ctx.Org.Organization.ID {
		ctx.APIError(http.StatusForbidden, "This rule belongs to a different organization")
		return
	}

	if err := actions_model.DeleteCrossRepoAccess(ctx, ruleID); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// Helper functions

func convertToAPIOrgPermissions(perm *actions_model.ActionOrgPermission) *api.OrgActionsPermissions {
	return &api.OrgActionsPermissions{
		PermissionMode:    int(perm.PermissionMode),
		AllowRepoOverride: perm.AllowRepoOverride,
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

func convertToCrossRepoAccessRule(rule *actions_model.ActionCrossRepoAccess) *api.CrossRepoAccessRule {
	return &api.CrossRepoAccessRule{
		ID:           rule.ID,
		OrgID:        rule.OrgID,
		SourceRepoID: rule.SourceRepoID,
		TargetRepoID: rule.TargetRepoID,
		AccessLevel:  rule.AccessLevel,
	}
}
