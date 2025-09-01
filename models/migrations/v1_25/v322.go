// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func CreateActionEnvironmentTable(x *xorm.Engine) error {
	type ActionEnvironment struct {
		ID          int64  `xorm:"pk autoincr"`
		RepoID      int64  `xorm:"INDEX UNIQUE(repo_name) NOT NULL"`
		Name        string `xorm:"UNIQUE(repo_name) NOT NULL"`
		Description string `xorm:"TEXT"`
		ExternalURL string `xorm:"TEXT"`

		// Protection rules as JSON
		ProtectionRules string `xorm:"LONGTEXT"`

		// Audit fields
		CreatedByID int64              `xorm:"INDEX"`
		CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
		UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
	}

	return x.Sync(new(ActionEnvironment))
}
