// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"xorm.io/xorm"
)

func AddIndeciesToIssueDepencencies(x *xorm.Engine) error {
	type IssueDependency struct {
		IssueID      int64 `xorm:"UNIQUE(issue_dependency) NOT NULL index"`
		DependencyID int64 `xorm:"UNIQUE(issue_dependency) NOT NULL index"`
	}

	return x.Sync(&IssueDependency{})
}
