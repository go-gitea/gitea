// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/require"
)

func TestAddRecipientAccessGrantedToRepoTransfer(t *testing.T) {
	type RepoTransfer struct {
		ID int64 `xorm:"pk autoincr"`
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(RepoTransfer))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(&RepoTransfer{})
	require.NoError(t, err)
	require.NoError(t, AddRecipientAccessGrantedToRepoTransfer(t.Context(), x))

	var got struct{ RecipientAccessGranted bool }
	has, err := x.Table("repo_transfer").Get(&got)
	require.NoError(t, err)
	require.True(t, has)
	require.False(t, got.RecipientAccessGranted)
}
