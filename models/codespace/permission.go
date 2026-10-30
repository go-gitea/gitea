// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	"gitea.dev/models/unit"
)

// PermissionAuthorization records a user's approval of additional repository access requested by a Codespace configuration.
type PermissionAuthorization struct {
	ID           int64
	UserID       int64  `xorm:"NOT NULL index(user_source_request)"`
	SourceRepoID int64  `xorm:"NOT NULL index(user_source_request)"`
	RequestHash  string `xorm:"CHAR(64) NOT NULL index(user_source_request)"`
	RevokedUnix  int64  `xorm:"NOT NULL DEFAULT 0"`
	CreatedUnix  int64  `xorm:"NOT NULL DEFAULT 0"`
	UpdatedUnix  int64  `xorm:"NOT NULL DEFAULT 0"`
}

// PermissionRepository stores one approved repository unit permission.
type PermissionRepository struct {
	AuthorizationID int64           `xorm:"pk NOT NULL"`
	TargetRepoID    int64           `xorm:"pk NOT NULL index"`
	UnitType        unit.Type       `xorm:"pk NOT NULL"`
	RequestedMode   perm.AccessMode `xorm:"NOT NULL"`
	GrantedMode     perm.AccessMode `xorm:"NOT NULL"`
}

func (*PermissionAuthorization) TableName() string {
	return "codespace_permission_authorization"
}

func (*PermissionRepository) TableName() string {
	return "codespace_permission_repository"
}

func init() {
	db.RegisterModel(new(PermissionAuthorization))
	db.RegisterModel(new(PermissionRepository))
}
