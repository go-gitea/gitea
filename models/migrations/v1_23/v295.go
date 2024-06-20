// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddRequireActionTable(x *xorm.Engine) error {
	type RequireAction struct {
		ID           int64              `xorm:"pk autoincr"`
		OrgID        int64              `xorm:"index"`
		RepoName     string             `xorm:"VARCHAR(255)"`
		WorkflowName string             `xorm:"VARCHAR(255) UNIQUE(require_action) NOT NULL"`
		CreatedUnix  timeutil.TimeStamp `xorm:"created NOT NULL"`
		UpdatedUnix  timeutil.TimeStamp `xorm:"updated"`
	}
	return x.Sync(new(RequireAction))
}
