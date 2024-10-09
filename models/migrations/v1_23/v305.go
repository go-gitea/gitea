// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddRepositoryLicenses(x *xorm.Engine) error {
	type RepoLicense struct {
		ID          int64 `xorm:"pk autoincr"`
		RepoID      int64 `xorm:"UNIQUE(s) NOT NULL"`
		CommitID    string
		License     string             `xorm:"VARCHAR(255) UNIQUE(s) NOT NULL"`
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX CREATED"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX UPDATED"`
	}

	return x.Sync(new(RepoLicense))
}
