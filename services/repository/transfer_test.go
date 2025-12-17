// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"sync"
	"testing"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/feed"
	notify_service "code.gitea.io/gitea/services/notify"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.NoError(t, repo.LoadOwner(t.Context()))
	repoTransfer := unittest.AssertExistsAndLoadBean(t, &repo_model.RepoTransfer{ID: 1})
	assert.NoError(t, repoTransfer.LoadAttributes(t.Context()))
	assert.NoError(t, AcceptTransferOwnership(t.Context(), repo, doer))

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
	assert.NoError(t, repo.LoadOwner(t.Context()))

	hasAccess, err := access_model.HasAnyUnitAccess(t.Context(), recipient.ID, repo)
	assert.NoError(t, err)
	assert.False(t, hasAccess)

	assert.NoError(t, StartRepositoryTransfer(t.Context(), doer, recipient, repo, nil))

	hasAccess, err = access_model.HasAnyUnitAccess(t.Context(), recipient.ID, repo)
	assert.NoError(t, err)
	assert.True(t, hasAccess)

	unittest.CheckConsistencyFor(t, &repo_model.Repository{}, &user_model.User{}, &organization.Team{})
}

func TestRepositoryTransfer(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})

	transfer, err := repo_model.GetPendingRepositoryTransfer(t.Context(), repo)
	assert.NoError(t, err)
	assert.NotNil(t, transfer)

	// Cancel transfer
	assert.NoError(t, CancelRepositoryTransfer(t.Context(), transfer, doer))

	transfer, err = repo_model.GetPendingRepositoryTransfer(t.Context(), repo)
	assert.Error(t, err)
	assert.Nil(t, transfer)
	assert.True(t, repo_model.IsErrNoPendingTransfer(err))

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	assert.NoError(t, repo_model.CreatePendingRepositoryTransfer(t.Context(), doer, user2, repo.ID, nil))

	transfer, err = repo_model.GetPendingRepositoryTransfer(t.Context(), repo)
	assert.NoError(t, err)
	assert.NoError(t, transfer.LoadAttributes(t.Context()))
	assert.Equal(t, "user2", transfer.Recipient.Name)

	org6 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// Only transfer can be started at any given time
	err = repo_model.CreatePendingRepositoryTransfer(t.Context(), doer, org6, repo.ID, nil)
	assert.Error(t, err)
	assert.True(t, repo_model.IsErrRepoTransferInProgress(err))

	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	// Unknown user, transfer non-existent transfer repo id = 2
	err = repo_model.CreatePendingRepositoryTransfer(t.Context(), doer, &user_model.User{ID: 1000, LowerName: "user1000"}, repo2.ID, nil)
	assert.Error(t, err)

	// Reject transfer
	err = RejectRepositoryTransfer(t.Context(), repo2, doer)
	assert.True(t, repo_model.IsErrNoPendingTransfer(err))
}

// Test transfer rejections
func TestRepositoryTransferRejection(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	// Set limit to 0 repositories so no repositories can be transferred
	defer test.MockVariableValue(&setting.Repository.MaxCreationLimit, 0)()

	// Admin case
	doerAdmin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 5})

	transfer, err := repo_model.GetPendingRepositoryTransfer(t.Context(), repo)
	require.NoError(t, err)
	require.NotNil(t, transfer)
	require.NoError(t, transfer.LoadRecipient(t.Context()))

	require.True(t, doerAdmin.CanCreateRepoIn(transfer.Recipient)) // admin is not subject to limits

	// Administrator should not be affected by the limits so transfer should be successful
	assert.NoError(t, AcceptTransferOwnership(t.Context(), repo, doerAdmin))

	// Non admin user case
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 10})
	repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 21})

	transfer, err = repo_model.GetPendingRepositoryTransfer(t.Context(), repo)
	require.NoError(t, err)
	require.NotNil(t, transfer)
	require.NoError(t, transfer.LoadRecipient(t.Context()))

	require.False(t, doer.CanCreateRepoIn(transfer.Recipient)) // regular user is subject to limits

	// Cannot accept because of the limit
	err = AcceptTransferOwnership(t.Context(), repo, doer)
	assert.Error(t, err)
	assert.True(t, IsRepositoryLimitReached(err))
}
