// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestRepositoryTransfer(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3}).(*user_model.User)
	repo := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 3}).(*Repository)

	transfer, err := GetPendingRepositoryTransfer(repo)
	assert.NoError(t, err)
	assert.NotNil(t, transfer)

	// Cancel transfer
	assert.NoError(t, CancelRepositoryTransfer(repo))

	transfer, err = GetPendingRepositoryTransfer(repo)
	assert.Error(t, err)
	assert.Nil(t, transfer)
	assert.True(t, IsErrNoPendingTransfer(err))

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)

	assert.NoError(t, CreatePendingRepositoryTransfer(doer, user2, repo.ID, nil))

	transfer, err = GetPendingRepositoryTransfer(repo)
	assert.Nil(t, err)
	assert.NoError(t, transfer.LoadAttributes())
	assert.Equal(t, "user2", transfer.Recipient.Name)

	user6 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)

	// Only transfer can be started at any given time
	err = CreatePendingRepositoryTransfer(doer, user6, repo.ID, nil)
	assert.Error(t, err)
	assert.True(t, IsErrRepoTransferInProgress(err))

	// Unknown user
	err = CreatePendingRepositoryTransfer(doer, &user_model.User{ID: 1000, LowerName: "user1000"}, repo.ID, nil)
	assert.Error(t, err)

	// Cancel transfer
	assert.NoError(t, CancelRepositoryTransfer(repo))
}
