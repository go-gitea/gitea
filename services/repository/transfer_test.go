// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"sync"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/notification/action"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

var notifySync sync.Once

func registerNotifier() {
	notifySync.Do(func() {
		notification.RegisterNotifier(action.NewNotifier())
	})
}

func TestTransferOwnership(t *testing.T) {
	registerNotifier()

	assert.NoError(t, unittest.PrepareTestDatabase())

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3}).(*repo_model.Repository)
	repo.Owner = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID}).(*user_model.User)
	assert.NoError(t, TransferOwnership(doer, doer, repo, nil))

	transferredRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3}).(*repo_model.Repository)
	assert.EqualValues(t, 2, transferredRepo.OwnerID)

	exist, err := util.IsExist(repo_model.RepoPath("user3", "repo3"))
	assert.NoError(t, err)
	assert.False(t, exist)
	exist, err = util.IsExist(repo_model.RepoPath("user2", "repo3"))
	assert.NoError(t, err)
	assert.True(t, exist)
	unittest.AssertExistsAndLoadBean(t, &models.Action{
		OpType:    models.ActionTransferRepo,
		ActUserID: 2,
		RepoID:    3,
		Content:   "user3/repo3",
	})

	unittest.CheckConsistencyFor(t, &repo_model.Repository{}, &user_model.User{}, &organization.Team{})
}

func TestStartRepositoryTransferSetPermission(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3}).(*user_model.User)
	recipient := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5}).(*user_model.User)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3}).(*repo_model.Repository)
	repo.Owner = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID}).(*user_model.User)

	hasAccess, err := access_model.HasAccess(db.DefaultContext, recipient.ID, repo)
	assert.NoError(t, err)
	assert.False(t, hasAccess)

	assert.NoError(t, StartRepositoryTransfer(doer, recipient, repo, nil))

	hasAccess, err = access_model.HasAccess(db.DefaultContext, recipient.ID, repo)
	assert.NoError(t, err)
	assert.True(t, hasAccess)

	unittest.CheckConsistencyFor(t, &repo_model.Repository{}, &user_model.User{}, &organization.Team{})
}
