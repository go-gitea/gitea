// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	issues_model "code.gitea.io/gitea/models/issues"
	"xorm.io/xorm"
)

func AddClosedStatusToIssue(x *xorm.Engine) error {
	type Issue struct {
		ClosedStatus int8
	}

	if err := x.Sync(new(Issue)); err != nil {
		return err
	}

	// TODO: TBD Whether to use IssueClosedStatusUndefined
	if _, err := x.Exec("UPDATE issue SET closed_status = ? WHERE closed_status IS NULL and is_pull = false AND is_closed = true", issues_model.IssueClosedStatusCommonClosed); err != nil {
		return err
	}

	_, err := x.Exec("UPDATE issue SET closed_status = ? WHERE closed_status IS NULL and is_pull = false AND is_closed = false", issues_model.IssueClosedStatusOpen)
	return err
}
