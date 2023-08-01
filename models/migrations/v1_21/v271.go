// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	issues_model "code.gitea.io/gitea/models/issues"

	"xorm.io/xorm"
)

func AddClosedStatusToIssue(x *xorm.Engine) error {
	type Issue struct {
		ClosedStatus issues_model.IssueClosedStatus `xorm:"NOT NULL DEFAULT 0"`
	}

	return x.Sync(new(Issue))
}
