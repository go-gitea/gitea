// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"sync"
	"testing"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/feed"
	notify_service "code.gitea.io/gitea/services/notify"

	"github.com/stretchr/testify/assert"
)

var notifySync sync.Once

func registerNotifier() {
	notifySync.Do(func() {
		notify_service.RegisterNotifier(feed.NewNotifier())
	})
}

func TestTransferOwnership(t *testing.T) {
	registerNotifier()

	assert.NoError(t, unittest.PrepareTestDatabase())

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	assert.NoError(t, repo.LoadOwner(db.DefaultContext))
	repoTransfer := unittest.AssertExistsAndLoadBean(t, &repo_model.RepoTransfer{ID: 1})
	assert.NoError(t, repoTransfer.LoadAttributes(db.DefaultContext))
	assert.NoError(t, AcceptTransferOwnership(db.DefaultContext, repo, doer))

	transferredRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	assert.EqualValues(t, 1, transferredRepo.OwnerID) // repo_transfer.yml id=1
	unittest.AssertNotExistsBean(t, &repo_model.RepoTransfer{ID: 1})

	exist, err := util.IsExist(repo_model.RepoPath("org3", "repo3"))
	assert.NoError(t, err)
	assert.False(t, exist)
	exist, err = util.IsExist(repo_model.RepoPath("user1", "repo3"))
	assert.NoError(t, err)
	assert.True(t, exist)
	unittest.AssertExistsAndLoadBean(t, &activities_model.Action{
		OpType:    activities_model.ActionTransferRepo,
		ActUserID: 1,
		RepoID:    3,
		Content:   "org3/repo3",
	})

	unittest.CheckConsistencyFor(t, &repo_model.Repository{}, &user_model.User{}, &organization.Team{})
}

func TestStartRepositoryTransferSetPermission(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	recipient := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	assert.NoError(t, repo.LoadOwner(db.DefaultContext))

	hasAccess, err := access_model.HasAnyUnitAccess(db.DefaultContext, recipient.ID, repo)
	assert.NoError(t, err)
	assert.False(t, hasAccess)

	assert.NoError(t, StartRepositoryTransfer(db.DefaultContext, doer, recipient, repo, nil))

	hasAccess, err = access_model.HasAnyUnitAccess(db.DefaultContext, recipient.ID, repo)
	assert.NoError(t, err)
	assert.True(t, hasAccess)

	unittest.CheckConsistencyFor(t, &repo_model.Repository{}, &user_model.User{}, &organization.Team{})
}

func TestRepositoryTransfer(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})

	transfer, err := repo_model.GetPendingRepositoryTransfer(db.DefaultContext, repo)
	assert.NoError(t, err)
	assert.NotNil(t, transfer)

	// Cancel transfer
	assert.NoError(t, CancelRepositoryTransfer(db.DefaultContext, transfer, doer))

	transfer, err = repo_model.GetPendingRepositoryTransfer(db.DefaultContext, repo)
	assert.Error(t, err)
	assert.Nil(t, transfer)
	assert.True(t, repo_model.IsErrNoPendingTransfer(err))

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	assert.NoError(t, repo_model.CreatePendingRepositoryTransfer(db.DefaultContext, doer, user2, repo.ID, nil))

	transfer, err = repo_model.GetPendingRepositoryTransfer(db.DefaultContext, repo)
	assert.NoError(t, err)
	assert.NoError(t, transfer.LoadAttributes(db.DefaultContext))
	assert.Equal(t, "user2", transfer.Recipient.Name)

	org6 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// Only transfer can be started at any given time
	err = repo_model.CreatePendingRepositoryTransfer(db.DefaultContext, doer, org6, repo.ID, nil)
	assert.Error(t, err)
	assert.True(t, repo_model.IsErrRepoTransferInProgress(err))

	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	// Unknown user, transfer non-existent transfer repo id = 2
	err = repo_model.CreatePendingRepositoryTransfer(db.DefaultContext, doer, &user_model.User{ID: 1000, LowerName: "user1000"}, repo2.ID, nil)
	assert.Error(t, err)

	// Reject transfer
	err = RejectRepositoryTransfer(db.DefaultContext, repo2, doer)
	assert.True(t, repo_model.IsErrNoPendingTransfer(err))
}
