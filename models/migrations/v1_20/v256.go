// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"xorm.io/xorm"
)

func AddIsInternalColumnToPackage(x *xorm.Engine) error {
	type Package struct {
		ID               int64  `xorm:"pk autoincr"`
		OwnerID          int64  `xorm:"UNIQUE(s) INDEX NOT NULL"`
		RepoID           int64  `xorm:"INDEX"`
		Type             string `xorm:"UNIQUE(s) INDEX NOT NULL"`
		Name             string `xorm:"NOT NULL"`
		LowerName        string `xorm:"UNIQUE(s) INDEX NOT NULL"`
		SemverCompatible bool   `xorm:"NOT NULL DEFAULT false"`
		IsInternal       bool   `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(Package))
}
