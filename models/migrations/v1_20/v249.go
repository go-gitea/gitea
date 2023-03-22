// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"xorm.io/xorm"
)

func AddClosedStatusToIssue(x *xorm.Engine) error {
	type Issue struct {
		ClosedStatus int8
	}

	if err := x.Sync(new(Issue)); err != nil {
		return err
	}

	// TODO: TBD Whether to use issues_model.IssueClosedStatusUndefined (-1)
	if _, err := x.Exec("UPDATE issue SET closed_status = ? WHERE closed_status IS NULL and is_pull = false AND is_closed = true", 1); err != nil {
		return err
	}

	_, err := x.Exec("UPDATE issue SET closed_status = ? WHERE closed_status IS NULL and is_pull = false AND is_closed = false", 0)
	return err
}
