// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestRepositoryTransfer(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})

	transfer, err := GetPendingRepositoryTransfer(db.DefaultContext, repo)
	assert.NoError(t, err)
	assert.NotNil(t, transfer)

	// Cancel transfer
	assert.NoError(t, CancelRepositoryTransfer(db.DefaultContext, repo))

	transfer, err = GetPendingRepositoryTransfer(db.DefaultContext, repo)
	assert.Error(t, err)
	assert.Nil(t, transfer)
	assert.True(t, IsErrNoPendingTransfer(err))

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	assert.NoError(t, CreatePendingRepositoryTransfer(db.DefaultContext, doer, user2, repo.ID, nil))

	transfer, err = GetPendingRepositoryTransfer(db.DefaultContext, repo)
	assert.Nil(t, err)
	assert.NoError(t, transfer.LoadAttributes(db.DefaultContext))
	assert.Equal(t, "user2", transfer.Recipient.Name)

	org6 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// Only transfer can be started at any given time
	err = CreatePendingRepositoryTransfer(db.DefaultContext, doer, org6, repo.ID, nil)
	assert.Error(t, err)
	assert.True(t, IsErrRepoTransferInProgress(err))

	// Unknown user
	err = CreatePendingRepositoryTransfer(db.DefaultContext, doer, &user_model.User{ID: 1000, LowerName: "user1000"}, repo.ID, nil)
	assert.Error(t, err)

	// Cancel transfer
	assert.NoError(t, CancelRepositoryTransfer(db.DefaultContext, repo))
}
