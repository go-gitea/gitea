// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"
)

func Test_AddCombinedIndexToIssueUser(t *testing.T) {
	type IssueUser struct {
		UID     int64 `xorm:"INDEX unique(uid_to_issue)"` // User ID.
		IssueID int64 `xorm:"INDEX unique(uid_to_issue)"`
	}

	// Prepare and load the testing database
	x, deferable := base.PrepareTestEnv(t, 0, new(IssueUser))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	if err := AddCombinedIndexToIssueUser(x); err != nil {
		t.Fatal(err)
	}
}
