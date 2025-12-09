// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"
	"xorm.io/xorm"
)

// ActionTokenPermission represents the permissions configuration for Actions tokens at repository level
type ActionTokenPermission struct {
	ID     int64 `xorm:"pk autoincr"`
	RepoID int64 `xorm:"UNIQUE NOT NULL"`

	// Permission mode: 0=restricted (default), 1=permissive, 2=custom
	PermissionMode int `xorm:"NOT NULL DEFAULT 0"`

	// Individual permission flags (only used when PermissionMode=2/custom)
	ActionsRead       bool `xorm:"NOT NULL DEFAULT false"`
	ActionsWrite      bool `xorm:"NOT NULL DEFAULT false"`
	ContentsRead      bool `xorm:"NOT NULL DEFAULT true"` // Always true for basic functionality
	ContentsWrite     bool `xorm:"NOT NULL DEFAULT false"`
	IssuesRead        bool `xorm:"NOT NULL DEFAULT false"`
	IssuesWrite       bool `xorm:"NOT NULL DEFAULT false"`
	PackagesRead      bool `xorm:"NOT NULL DEFAULT false"`
	PackagesWrite     bool `xorm:"NOT NULL DEFAULT false"`
	PullRequestsRead  bool `xorm:"NOT NULL DEFAULT false"`
	PullRequestsWrite bool `xorm:"NOT NULL DEFAULT false"`
	MetadataRead      bool `xorm:"NOT NULL DEFAULT true"` // Always true

	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

// ActionOrgPermission represents the permissions configuration for Actions tokens at organization level
type ActionOrgPermission struct {
	ID    int64 `xorm:"pk autoincr"`
	OrgID int64 `xorm:"UNIQUE NOT NULL"`

	// Permission mode: 0=restricted (default), 1=permissive, 2=custom
	PermissionMode int `xorm:"NOT NULL DEFAULT 0"`

	// Whether repos can override (set their own permissions)
	// If false, all repos must use org settings
	AllowRepoOverride bool `xorm:"NOT NULL DEFAULT true"`

	// Individual permission flags (only used when PermissionMode=2/custom)
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

// ActionCrossRepoAccess represents cross-repository access rules within an organization
type ActionCrossRepoAccess struct {
	ID           int64 `xorm:"pk autoincr"`
	OrgID        int64 `xorm:"INDEX NOT NULL"`
	SourceRepoID int64 `xorm:"INDEX NOT NULL"` // Repo that wants to access
	TargetRepoID int64 `xorm:"INDEX NOT NULL"` // Repo being accessed

	// Access level: 0=none, 1=read, 2=write
	AccessLevel int `xorm:"NOT NULL DEFAULT 0"`

	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}

// PackageRepoLink links packages to repositories for permission checking
type PackageRepoLink struct {
	ID        int64 `xorm:"pk autoincr"`
	PackageID int64 `xorm:"INDEX NOT NULL"`
	RepoID    int64 `xorm:"INDEX NOT NULL"`

	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}

func AddActionsPermissionsTables(x *xorm.Engine) error {
	// Create action_token_permission table
	if err := x.Sync2(new(ActionTokenPermission)); err != nil {
		return err
	}

	// Create action_org_permission table
	if err := x.Sync2(new(ActionOrgPermission)); err != nil {
		return err
	}

	// Create action_cross_repo_access table
	if err := x.Sync2(new(ActionCrossRepoAccess)); err != nil {
		return err
	}

	// Create package_repo_link table
	if err := x.Sync2(new(PackageRepoLink)); err != nil {
		return err
	}

	return nil
}
