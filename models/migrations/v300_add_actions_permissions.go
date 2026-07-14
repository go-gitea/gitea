// Package migrations provides database migrations.
// Migration to create tables for configurable Actions token permissions. Modified by LAC | Ludwig investing
package migrations

import (
	"code.gitea.io/gitea/models/migrations/base"
	"xorm.io/xorm"
)

func init() {
	base.RegisterMigration("Add actions permissions tables", addActionsPermissionsTables)
}

func addActionsPermissionsTables(x *xorm.Engine) error {
	type RepoActionsPermissions struct {
		ID          int64  `xorm:"pk autoincr"`
		RepoID      int64  `xorm:"UNIQUE NOT NULL"`
		Permissions string `xorm:"TEXT"`
	}
	type OrgActionsPermissions struct {
		ID          int64  `xorm:"pk autoincr"`
		OrgID       int64  `xorm:"UNIQUE NOT NULL"`
		Permissions string `xorm:"TEXT"`
	}
	type RepoActionsAccess struct {
		ID           int64  `xorm:"pk autoincr"`
		OrgID        int64  `xorm:"NOT NULL"`
		SourceRepoID int64  `xorm:"NOT NULL"`
		TargetRepoID int64  `xorm:"NOT NULL"`
		Permissions  string `xorm:"TEXT"`
	}
	type PackageActionsAccess struct {
		ID         int64  `xorm:"pk autoincr"`
		RepoID     int64  `xorm:"NOT NULL"`
		PackageID  int64  `xorm:"NOT NULL"`
		Permission string `xorm:"TEXT"`
	}
	return x.Sync2(new(RepoActionsPermissions), new(OrgActionsPermissions), new(RepoActionsAccess), new(PackageActionsAccess))
}
