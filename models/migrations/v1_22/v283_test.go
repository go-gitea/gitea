// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"

	"github.com/stretchr/testify/assert"
)

func Test_AddCombinedIndexToIssueUser(t *testing.T) {
	type IssueUser struct { // old struct
		ID          int64 `xorm:"pk autoincr"`
		UID         int64 `xorm:"INDEX"` // User ID.
		IssueID     int64 `xorm:"INDEX"`
		IsRead      bool
		IsMentioned bool
	}

	// Prepare and load the testing database
	x, deferable := base.PrepareTestEnv(t, 0, new(IssueUser))
	defer deferable()

	assert.NoError(t, AddCombinedIndexToIssueUser(x))
}
