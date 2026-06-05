// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"
)

type UserGroup struct {
	ID          int64              `xorm:"pk autoincr"`
	Name        string             `xorm:"NOT NULL"`
	LowerName   string             `xorm:"INDEX NOT NULL"`
	Slug        string             `xorm:"UNIQUE NOT NULL"`
	Description string             `xorm:"TEXT NOT NULL"`
	ParentID    int64              `xorm:"INDEX"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

type UserGroupMember struct {
	GroupID int64 `xorm:"UNIQUE(s) INDEX"`
	UserID  int64 `xorm:"UNIQUE(s) INDEX"`
}

type TeamUserGroup struct {
	TeamID  int64 `xorm:"UNIQUE(s) INDEX"`
	GroupID int64 `xorm:"UNIQUE(s) INDEX"`
	OrgID   int64 `xorm:"INDEX"`
}

func AddUserGroups(x db.EngineMigration) error {
	return x.Sync(new(UserGroup), new(UserGroupMember), new(TeamUserGroup))
}
