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

	"github.com/stretchr/testify/assert"
	"github.com/unknwon/com"
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
	assert.NoError(t, TransferOwnership(doer, "user2", repo))

	transferredRepo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 3}).(*models.Repository)
	assert.EqualValues(t, 2, transferredRepo.OwnerID)

	assert.False(t, com.IsExist(models.RepoPath("user3", "repo3")))
	assert.True(t, com.IsExist(models.RepoPath("user2", "repo3")))
	models.AssertExistsAndLoadBean(t, &models.Action{
		OpType:    models.ActionTransferRepo,
		ActUserID: 2,
		RepoID:    3,
		Content:   "user3/repo3",
	})

	models.CheckConsistencyFor(t, &models.Repository{}, &models.User{}, &models.Team{})
}
