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
		ID          int64 `xorm:"pk autoincr"`
		RecipientID int64
		RepoID      int64
	}
	type Collaboration struct {
		ID     int64 `xorm:"pk autoincr"`
		RepoID int64
		UserID int64
		Mode   int
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(RepoTransfer), new(Collaboration))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(
		&RepoTransfer{ID: 1, RecipientID: 2, RepoID: 1},
		&RepoTransfer{ID: 2, RecipientID: 3, RepoID: 2},
		&RepoTransfer{ID: 3, RecipientID: 4, RepoID: 3},
	)
	require.NoError(t, err)
	_, err = x.Insert(
		&Collaboration{RepoID: 1, UserID: 2, Mode: 1},
		&Collaboration{RepoID: 2, UserID: 3, Mode: 2},
	)
	require.NoError(t, err)
	require.NoError(t, AddRecipientAccessGrantedToRepoTransfer(t.Context(), x))

	var got []struct {
		ID                     int64
		RecipientAccessGranted bool
	}
	err = x.Table("repo_transfer").OrderBy("id").Find(&got)
	require.NoError(t, err)
	require.Equal(t, []struct {
		ID                     int64
		RecipientAccessGranted bool
	}{
		{ID: 1, RecipientAccessGranted: true},
		{ID: 2, RecipientAccessGranted: false},
		{ID: 3, RecipientAccessGranted: false},
	}, got)
}
