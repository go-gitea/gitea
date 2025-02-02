// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_6 //nolint

import (
	"fmt"

	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddReview(x *xorm.Engine) error {
	// Review see models/review.go
	type Review struct {
		ID          int64 `xorm:"pk autoincr"`
		Type        string
		ReviewerID  int64 `xorm:"index"`
		IssueID     int64 `xorm:"index"`
		Content     string
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	if err := x.Sync(new(Review)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
