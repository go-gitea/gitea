// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12 //nolint

import (
	"xorm.io/xorm"
)

func AddReviewCommitAndStale(x *xorm.Engine) error {
	type Review struct {
		CommitID string `xorm:"VARCHAR(40)"`
		Stale    bool   `xorm:"NOT NULL DEFAULT false"`
	}

	type ProtectedBranch struct {
		DismissStaleApprovals bool `xorm:"NOT NULL DEFAULT false"`
	}

	// Old reviews will have commit ID set to "" and not stale
	if err := x.Sync(new(Review)); err != nil {
		return err
	}
	return x.Sync(new(ProtectedBranch))
}
