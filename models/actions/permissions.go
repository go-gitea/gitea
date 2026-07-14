// Package actions provides database models for configurable Actions token permissions.
// Database models for configurable Actions token permissions. Modified by LAC | Ludwig investing
package actions

import (
	"context"
	"encoding/json"

	"code.gitea.io/gitea/models/db"
)

// RepoActionsPermissions stores the maximum permissions for a repository's Actions token.
type RepoActionsPermissions struct {
	ID          int64  `xorm:"pk autoincr"`
	RepoID      int64  `xorm:"UNIQUE NOT NULL"`
	Permissions string `xorm:"TEXT"` // JSON map of Scope to Permission
}

// TableName returns the table name.
func (RepoActionsPermissions) TableName() string {
	return "repo_actions_permissions"
}

// GetPermissionsMap decodes the JSON permissions into a map.
func (p *RepoActionsPermissions) GetPermissionsMap() (map[Scope]Permission, error) {
	var m map[Scope]Permission
	if err := json.Unmarshal([]byte(p.Permissions), &m); err != nil {
		return nil, err
	}
	return m, nil
}

// SetPermissionsMap encodes a permissions map to JSON and sets it.
func (p *RepoActionsPermissions) SetPermissionsMap(perms map[Scope]Permission) error {
	data, err := json.Marshal(perms)
	if err != nil {
		return err
	}
	p.Permissions = string(data)
	return nil
}

// GetRepoActionsPermissions returns the permissions for a repository, or nil if not set.
func GetRepoActionsPermissions(ctx context.Context, repoID int64) (*RepoActionsPermissions, error) {
	perm := &RepoActionsPermissions{}
	has, err := db.GetEngine(ctx).Where("repo_id = ?", repoID).Get(perm)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return perm, nil
}

// UpdateRepoActionsPermissions creates or updates the permissions for a repository.
func UpdateRepoActionsPermissions(ctx context.Context, repoID int64, perms map[Scope]Permission) error {
	p := &RepoActionsPermissions{RepoID: repoID}
	if err := p.SetPermissionsMap(perms); err != nil {
		return err
	}
	_, err := db.GetEngine(ctx).InsertOrUpdate(p)
	return err
}

// OrgActionsPermissions stores the default maximum permissions for an organization.
type OrgActionsPermissions struct {
	ID          int64  `xorm:"pk autoincr"`
	OrgID       int64  `xorm:"UNIQUE NOT NULL"`
	Permissions string `xorm:"TEXT"`
}

func (OrgActionsPermissions) TableName() string {
	return "org_actions_permissions"
}

func (p *OrgActionsPermissions) GetPermissionsMap() (map[Scope]Permission, error) {
	var m map[Scope]Permission
	if err := json.Unmarshal([]byte(p.Permissions), &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (p *OrgActionsPermissions) SetPermissionsMap(perms map[Scope]Permission) error {
	data, err := json.Marshal(perms)
	if err != nil {
		return err
	}
	p.Permissions = string(data)
	return nil
}

func GetOrgActionsPermissions(ctx context.Context, orgID int64) (*OrgActionsPermissions, error) {
	perm := &OrgActionsPermissions{}
	has, err := db.GetEngine(ctx).Where("org_id = ?", orgID).Get(perm)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return perm, nil
}

func UpdateOrgActionsPermissions(ctx context.Context, orgID int64, perms map[Scope]Permission) error {
	p := &OrgActionsPermissions{OrgID: orgID}
	if err := p.SetPermissionsMap(perms); err != nil {
		return err
	}
	_, err := db.GetEngine(ctx).InsertOrUpdate(p)
	return err
}

// RepoActionsAccess defines cross-repo access permissions within an organization.
type RepoActionsAccess struct {
	ID           int64  `xorm:"pk autoincr"`
	OrgID        int64  `xorm:"NOT NULL"`
	SourceRepoID int64  `xorm:"NOT NULL"`
	TargetRepoID int64  `xorm:"NOT NULL"`
	Permissions  string `xorm:"TEXT"` // JSON map of Scope to Permission
}

func (RepoActionsAccess) TableName() string {
	return "repo_actions_access"
}

func (a *RepoActionsAccess) GetPermissionsMap() (map[Scope]Permission, error) {
	var m map[Scope]Permission
	if err := json.Unmarshal([]byte(a.Permissions), &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (a *RepoActionsAccess) SetPermissionsMap(perms map[Scope]Permission) error {
	data, err := json.Marshal(perms)
	if err != nil {
		return err
	}
	a.Permissions = string(data)
	return nil
}

// GetCrossRepoAccess returns all access rules where the given repo is the source.
func GetCrossRepoAccess(ctx context.Context, sourceRepoID int64) ([]*RepoActionsAccess, error) {
	accesses := make([]*RepoActionsAccess, 0)
	err := db.GetEngine(ctx).Where("source_repo_id = ?", sourceRepoID).Find(&accesses)
	return accesses, err
}

// SetCrossRepoAccess creates or updates a cross-repo access rule.
func SetCrossRepoAccess(ctx context.Context, orgID, sourceRepoID, targetRepoID int64, perms map[Scope]Permission) error {
	a := &RepoActionsAccess{
		OrgID:        orgID,
		SourceRepoID: sourceRepoID,
		TargetRepoID: targetRepoID,
	}
	if err := a.SetPermissionsMap(perms); err != nil {
		return err
	}
	_, err := db.GetEngine(ctx).InsertOrUpdate(a)
	return err
}

// PackageActionsAccess defines which packages a repository can access via Actions.
type PackageActionsAccess struct {
	ID         int64  `xorm:"pk autoincr"`
	RepoID     int64  `xorm:"NOT NULL"`
	PackageID  int64  `xorm:"NOT NULL"`
	Permission string `xorm:"TEXT"` // "read" or "write"
}

func (PackageActionsAccess) TableName() string {
	return "package_actions_access"
}

// GetPackageAccess returns the access permissions for a repository to packages.
func GetPackageAccess(ctx context.Context, repoID int64) ([]*PackageActionsAccess, error) {
	accesses := make([]*PackageActionsAccess, 0)
	err := db.GetEngine(ctx).Where("repo_id = ?", repoID).Find(&accesses)
	return accesses, err
}

// SetPackageAccess sets the access permission for a repository to a package.
func SetPackageAccess(ctx context.Context, repoID, packageID int64, permission string) error {
	a := &PackageActionsAccess{
		RepoID:     repoID,
		PackageID:  packageID,
		Permission: permission,
	}
	_, err := db.GetEngine(ctx).InsertOrUpdate(a)
	return err
}
