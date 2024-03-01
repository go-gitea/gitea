// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16 //nolint

import (
	"xorm.io/xorm"
)

func AddProjectIssueSorting(x *xorm.Engine) error {
	// ProjectIssue saves relation from issue to a project
	type ProjectIssue struct {
		Sorting int64 `xorm:"NOT NULL DEFAULT 0"`
	}

	return x.Sync(new(ProjectIssue))
}
