// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"sync"
	"testing"

	"code.gitea.io/gitea/models"
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

	assert.NoError(t, models.PrepareTestDatabase())

	doer := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 3}).(*models.Repository)
	repo.Owner = models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)
	assert.NoError(t, TransferOwnership(doer, doer, repo, nil))

	transferredRepo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 3}).(*models.Repository)
	assert.EqualValues(t, 2, transferredRepo.OwnerID)

	exist, err := util.IsExist(models.RepoPath("user3", "repo3"))
	assert.NoError(t, err)
	assert.False(t, exist)
	exist, err = util.IsExist(models.RepoPath("user2", "repo3"))
	assert.NoError(t, err)
	assert.True(t, exist)
	models.AssertExistsAndLoadBean(t, &models.Action{
		OpType:    models.ActionTransferRepo,
		ActUserID: 2,
		RepoID:    3,
		Content:   "user3/repo3",
	})

	models.CheckConsistencyFor(t, &models.Repository{}, &models.User{}, &models.Team{})
}

func TestStartRepositoryTransferSetPermission(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	doer := models.AssertExistsAndLoadBean(t, &models.User{ID: 3}).(*models.User)
	recipient := models.AssertExistsAndLoadBean(t, &models.User{ID: 5}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 3}).(*models.Repository)
	repo.Owner = models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	hasAccess, err := models.HasAccess(recipient.ID, repo)
	assert.NoError(t, err)
	assert.False(t, hasAccess)

	assert.NoError(t, StartRepositoryTransfer(doer, recipient, repo, nil))

	hasAccess, err = models.HasAccess(recipient.ID, repo)
	assert.NoError(t, err)
	assert.True(t, hasAccess)

	models.CheckConsistencyFor(t, &models.Repository{}, &models.User{}, &models.Team{})
}
