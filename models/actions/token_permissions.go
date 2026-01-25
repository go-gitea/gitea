// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/util"
)

// ActionTokenPermissions represents the permissions configuration for Actions job tokens
type ActionTokenPermissions struct {
	ID     int64 `xorm:"pk autoincr"`
	RepoID int64 `xorm:"UNIQUE(repo_org) INDEX NOT NULL"`
	OrgID  int64 `xorm:"UNIQUE(repo_org) INDEX NOT NULL DEFAULT 0"`

	// DefaultPermissions is the default permission level for job tokens
	// Can be "read", "write", or "none"
	DefaultPermissions string `xorm:"VARCHAR(20) NOT NULL DEFAULT 'read'"`

	// Specific scope permissions (stored as comma-separated AccessTokenScope values)
	// These override the default permissions for specific categories
	ContentsPermission      string `xorm:"VARCHAR(20) NOT NULL DEFAULT ''"`
	IssuesPermission        string `xorm:"VARCHAR(20) NOT NULL DEFAULT ''"`
	PullRequestsPermission  string `xorm:"VARCHAR(20) NOT NULL DEFAULT ''"`
	PackagesPermission      string `xorm:"VARCHAR(20) NOT NULL DEFAULT ''"`
	MetadataPermission      string `xorm:"VARCHAR(20) NOT NULL DEFAULT 'read'"` // always at least read
	ActionsPermission       string `xorm:"VARCHAR(20) NOT NULL DEFAULT ''"`
	OrganizationPermission  string `xorm:"VARCHAR(20) NOT NULL DEFAULT ''"`
	NotificationPermission  string `xorm:"VARCHAR(20) NOT NULL DEFAULT ''"`
}

func init() {
	db.RegisterModel(new(ActionTokenPermissions))
}

// GetActionTokenPermissions gets the token permissions for a repository
func GetActionTokenPermissions(ctx context.Context, repoID int64) (*ActionTokenPermissions, error) {
	perms := &ActionTokenPermissions{RepoID: repoID}
	has, err := db.GetEngine(ctx).Get(perms)
	if err != nil {
		return nil, err
	}
	if !has {
		// Return default permissions if not configured
		return &ActionTokenPermissions{
			RepoID:             repoID,
			DefaultPermissions: "read",
			MetadataPermission: "read",
		}, nil
	}
	return perms, nil
}

// GetActionTokenPermissionsByOrg gets the default token permissions for an organization
func GetActionTokenPermissionsByOrg(ctx context.Context, orgID int64) (*ActionTokenPermissions, error) {
	perms := &ActionTokenPermissions{OrgID: orgID, RepoID: 0}
	has, err := db.GetEngine(ctx).Get(perms)
	if err != nil {
		return nil, err
	}
	if !has {
		// Return default permissions if not configured
		return &ActionTokenPermissions{
			OrgID:              orgID,
			DefaultPermissions: "read",
			MetadataPermission: "read",
		}, nil
	}
	return perms, nil
}

// SetActionTokenPermissions creates or updates token permissions for a repository
func SetActionTokenPermissions(ctx context.Context, perms *ActionTokenPermissions) error {
	if perms.RepoID == 0 && perms.OrgID == 0 {
		return util.NewInvalidArgumentErrorf("either RepoID or OrgID must be set")
	}

	existing := &ActionTokenPermissions{}
	var has bool
	var err error

	if perms.RepoID > 0 {
		has, err = db.GetEngine(ctx).Where("repo_id = ?", perms.RepoID).Get(existing)
	} else {
		has, err = db.GetEngine(ctx).Where("org_id = ? AND repo_id = 0", perms.OrgID).Get(existing)
	}

	if err != nil {
		return err
	}

	if has {
		perms.ID = existing.ID
		_, err = db.GetEngine(ctx).ID(perms.ID).AllCols().Update(perms)
		return err
	}

	_, err = db.GetEngine(ctx).Insert(perms)
	return err
}

// ToAccessTokenScopes converts the permissions to AccessTokenScope
func (p *ActionTokenPermissions) ToAccessTokenScopes() auth_model.AccessTokenScope {
	scopes := make([]string, 0)

	// Helper to add scope based on permission level
	addScope := func(permission string, category auth_model.AccessTokenScopeCategory) {
		if permission == "" {
			permission = p.DefaultPermissions
		}
		switch permission {
		case "read":
			scopes = append(scopes, string(auth_model.GetRequiredScopes(auth_model.Read, category)[0]))
		case "write":
			scopes = append(scopes, string(auth_model.GetRequiredScopes(auth_model.Write, category)[0]))
		case "none":
			// Don't add any scope
		}
	}

	// Add scopes for each category
	addScope(p.ContentsPermission, auth_model.AccessTokenScopeCategoryRepository)
	addScope(p.IssuesPermission, auth_model.AccessTokenScopeCategoryIssue)
	addScope(p.PullRequestsPermission, auth_model.AccessTokenScopeCategoryIssue) // PRs use issue scope
	addScope(p.PackagesPermission, auth_model.AccessTokenScopeCategoryPackage)
	addScope(p.OrganizationPermission, auth_model.AccessTokenScopeCategoryOrganization)
	addScope(p.NotificationPermission, auth_model.AccessTokenScopeCategoryNotification)

	// Metadata is always at least read
	if p.MetadataPermission == "" || p.MetadataPermission == "read" {
		scopes = append(scopes, string(auth_model.AccessTokenScopeReadRepository))
	}

	// Join all scopes
	if len(scopes) == 0 {
		return ""
	}

	scopeStr := ""
	for i, scope := range scopes {
		if i > 0 {
			scopeStr += ","
		}
		scopeStr += scope
	}

	return auth_model.AccessTokenScope(scopeStr)
}
