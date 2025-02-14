// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func CreatePackageCleanupRuleTable(x *xorm.Engine) error {
	type PackageCleanupRule struct {
		ID            int64              `xorm:"pk autoincr"`
		Enabled       bool               `xorm:"INDEX NOT NULL DEFAULT false"`
		OwnerID       int64              `xorm:"UNIQUE(s) INDEX NOT NULL DEFAULT 0"`
		Type          string             `xorm:"UNIQUE(s) INDEX NOT NULL"`
		KeepCount     int                `xorm:"NOT NULL DEFAULT 0"`
		KeepPattern   string             `xorm:"NOT NULL DEFAULT ''"`
		RemoveDays    int                `xorm:"NOT NULL DEFAULT 0"`
		RemovePattern string             `xorm:"NOT NULL DEFAULT ''"`
		MatchFullName bool               `xorm:"NOT NULL DEFAULT false"`
		CreatedUnix   timeutil.TimeStamp `xorm:"created NOT NULL DEFAULT 0"`
		UpdatedUnix   timeutil.TimeStamp `xorm:"updated NOT NULL DEFAULT 0"`
	}

	return x.Sync(new(PackageCleanupRule))
}
