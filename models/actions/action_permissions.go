// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"context"
)

// PermissionMode represents the permission configuration mode
type PermissionMode int

const (
	// PermissionModeRestricted - minimal permissions (default, secure)
	PermissionModeRestricted PermissionMode = 0

	// PermissionModePermissive - broad permissions (convenience)
	PermissionModePermissive PermissionMode = 1

	// PermissionModeCustom - user-defined permissions
	PermissionModeCustom PermissionMode = 2
)

// ActionTokenPermission represents repository-level Actions token permissions
type ActionTokenPermission struct {
	ID     int64 `xorm:"pk autoincr"`
	RepoID int64 `xorm:"UNIQUE NOT NULL"`

	PermissionMode PermissionMode `xorm:"NOT NULL DEFAULT 0"`

	// Granular permissions (only used in Custom mode)
	ActionsRead       bool `xorm:"NOT NULL DEFAULT false"`
	ActionsWrite      bool `xorm:"NOT NULL DEFAULT false"`
	ContentsRead      bool `xorm:"NOT NULL DEFAULT true"`
	ContentsWrite     bool `xorm:"NOT NULL DEFAULT false"`
	IssuesRead        bool `xorm:"NOT NULL DEFAULT false"`
	IssuesWrite       bool `xorm:"NOT NULL DEFAULT false"`
	PackagesRead      bool `xorm:"NOT NULL DEFAULT false"`
	PackagesWrite     bool `xorm:"NOT NULL DEFAULT false"`
	PullRequestsRead  bool `xorm:"NOT NULL DEFAULT false"`
	PullRequestsWrite bool `xorm:"NOT NULL DEFAULT false"`
	MetadataRead      bool `xorm:"NOT NULL DEFAULT true"`

	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

// ActionOrgPermission represents organization-level Actions token permissions
type ActionOrgPermission struct {
	ID    int64 `xorm:"pk autoincr"`
	OrgID int64 `xorm:"UNIQUE NOT NULL"`

	PermissionMode    PermissionMode `xorm:"NOT NULL DEFAULT 0"`
	AllowRepoOverride bool           `xorm:"NOT NULL DEFAULT true"`

	// Granular permissions (only used in Custom mode)
	ActionsRead       bool `xorm:"NOT NULL DEFAULT false"`
	ActionsWrite      bool `xorm:"NOT NULL DEFAULT false"`
	ContentsRead      bool `xorm:"NOT NULL DEFAULT true"`
	ContentsWrite     bool `xorm:"NOT NULL DEFAULT false"`
	IssuesRead        bool `xorm:"NOT NULL DEFAULT false"`
	IssuesWrite       bool `xorm:"NOT NULL DEFAULT false"`
	PackagesRead      bool `xorm:"NOT NULL DEFAULT false"`
	PackagesWrite     bool `xorm:"NOT NULL DEFAULT false"`
	PullRequestsRead  bool `xorm:"NOT NULL DEFAULT false"`
	PullRequestsWrite bool `xorm:"NOT NULL DEFAULT false"`
	MetadataRead      bool `xorm:"NOT NULL DEFAULT true"`

	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(ActionTokenPermission))
	db.RegisterModel(new(ActionOrgPermission))
}

// GetRepoActionPermissions retrieves the Actions permissions for a repository
// If no configuration exists, returns nil (will use defaults)
func GetRepoActionPermissions(ctx context.Context, repoID int64) (*ActionTokenPermission, error) {
	perm := &ActionTokenPermission{RepoID: repoID}
	has, err := db.GetEngine(ctx).Get(perm)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil // No custom config, will use defaults
	}
	return perm, nil
}

// GetOrgActionPermissions retrieves the Actions permissions for an organization
func GetOrgActionPermissions(ctx context.Context, orgID int64) (*ActionOrgPermission, error) {
	perm := &ActionOrgPermission{OrgID: orgID}
	has, err := db.GetEngine(ctx).Get(perm)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil // No custom config, will use defaults
	}
	return perm, nil
}

// CreateOrUpdateRepoPermissions creates or updates repository-level permissions
func CreateOrUpdateRepoPermissions(ctx context.Context, perm *ActionTokenPermission) error {
	existing := &ActionTokenPermission{RepoID: perm.RepoID}
	has, err := db.GetEngine(ctx).Get(existing)
	if err != nil {
		return err
	}

	if has {
		// Update existing
		perm.ID = existing.ID
		perm.CreatedUnix = existing.CreatedUnix
		_, err = db.GetEngine(ctx).ID(perm.ID).Update(perm)
		return err
	}

	// Create new
	_, err = db.GetEngine(ctx).Insert(perm)
	return err
}

// CreateOrUpdateOrgPermissions creates or updates organization-level permissions
func CreateOrUpdateOrgPermissions(ctx context.Context, perm *ActionOrgPermission) error {
	existing := &ActionOrgPermission{OrgID: perm.OrgID}
	has, err := db.GetEngine(ctx).Get(existing)
	if err != nil {
		return err
	}

	if has {
		// Update existing
		perm.ID = existing.ID
		perm.CreatedUnix = existing.CreatedUnix
		_, err = db.GetEngine(ctx).ID(perm.ID).Update(perm)
		return err
	}

	// Create new
	_, err = db.GetEngine(ctx).Insert(perm)
	return err
}

// ToPermissionMap converts permission struct to a map for easy access
func (p *ActionTokenPermission) ToPermissionMap() map[string]map[string]bool {
	// Apply permission mode defaults
	var perms map[string]map[string]bool

	switch p.PermissionMode {
	case PermissionModeRestricted:
		// Minimal permissions - only read metadata and contents
		perms = map[string]map[string]bool{
			"actions":       {"read": false, "write": false},
			"contents":      {"read": true, "write": false},
			"issues":        {"read": false, "write": false},
			"packages":      {"read": false, "write": false},
			"pull_requests": {"read": false, "write": false},
			"metadata":      {"read": true, "write": false},
		}
	case PermissionModePermissive:
		// Broad permissions - read/write for most things
		perms = map[string]map[string]bool{
			"actions":       {"read": true, "write": true},
			"contents":      {"read": true, "write": true},
			"issues":        {"read": true, "write": true},
			"packages":      {"read": true, "write": true},
			"pull_requests": {"read": true, "write": true},
			"metadata":      {"read": true, "write": false},
		}
	case PermissionModeCustom:
		// Use explicitly set permissions
		perms = map[string]map[string]bool{
			"actions":       {"read": p.ActionsRead, "write": p.ActionsWrite},
			"contents":      {"read": p.ContentsRead, "write": p.ContentsWrite},
			"issues":        {"read": p.IssuesRead, "write": p.IssuesWrite},
			"packages":      {"read": p.PackagesRead, "write": p.PackagesWrite},
			"pull_requests": {"read": p.PullRequestsRead, "write": p.PullRequestsWrite},
			"metadata":      {"read": p.MetadataRead, "write": false},
		}
	}

	return perms
}

// ToPermissionMap converts org permission struct to a map
func (p *ActionOrgPermission) ToPermissionMap() map[string]map[string]bool {
	var perms map[string]map[string]bool

	switch p.PermissionMode {
	case PermissionModeRestricted:
		perms = map[string]map[string]bool{
			"actions":       {"read": false, "write": false},
			"contents":      {"read": true, "write": false},
			"issues":        {"read": false, "write": false},
			"packages":      {"read": false, "write": false},
			"pull_requests": {"read": false, "write": false},
			"metadata":      {"read": true, "write": false},
		}
	case PermissionModePermissive:
		perms = map[string]map[string]bool{
			"actions":       {"read": true, "write": true},
			"contents":      {"read": true, "write": true},
			"issues":        {"read": true, "write": true},
			"packages":      {"read": true, "write": true},
			"pull_requests": {"read": true, "write": true},
			"metadata":      {"read": true, "write": false},
		}
	case PermissionModeCustom:
		perms = map[string]map[string]bool{
			"actions":       {"read": p.ActionsRead, "write": p.ActionsWrite},
			"contents":      {"read": p.ContentsRead, "write": p.ContentsWrite},
			"issues":        {"read": p.IssuesRead, "write": p.IssuesWrite},
			"packages":      {"read": p.PackagesRead, "write": p.PackagesWrite},
			"pull_requests": {"read": p.PullRequestsRead, "write": p.PullRequestsWrite},
			"metadata":      {"read": p.MetadataRead, "write": false},
		}
	}

	return perms
}
