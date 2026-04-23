// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"

	"github.com/stretchr/testify/require"
)

func Test_AddDismissApprovalsOnReRequestColumnToProtectedBranch(t *testing.T) {
	type ProtectedBranch struct {
		ID int64 `xorm:"pk autoincr"`
	}

	x, deferable := base.PrepareTestEnv(t, 0, new(ProtectedBranch))
	defer deferable()

	_, err := x.Insert(&ProtectedBranch{})
	require.NoError(t, err)

	require.NoError(t, AddDismissApprovalsOnReRequestColumnToProtectedBranch(x))

	var dismiss bool
	has, err := x.SQL("SELECT dismiss_approvals_on_re_request FROM protected_branch WHERE id = ?", 1).Get(&dismiss)
	require.NoError(t, err)
	require.True(t, has)
	require.False(t, dismiss)
}
