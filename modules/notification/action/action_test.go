// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package action

import (
	"path/filepath"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, filepath.Join("..", "..", ".."))
}

func TestRenameRepoAction(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := db.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := db.AssertExistsAndLoadBean(t, &models.Repository{OwnerID: user.ID}).(*models.Repository)
	repo.Owner = user

	oldRepoName := repo.Name
	const newRepoName = "newRepoName"
	repo.Name = newRepoName
	repo.LowerName = strings.ToLower(newRepoName)

	actionBean := &models.Action{
		OpType:    models.ActionRenameRepo,
		ActUserID: user.ID,
		ActUser:   user,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
		Content:   oldRepoName,
	}
	db.AssertNotExistsBean(t, actionBean)

	NewNotifier().NotifyRenameRepository(user, repo, oldRepoName)

	db.AssertExistsAndLoadBean(t, actionBean)
	models.CheckConsistencyFor(t, &models.Action{})
}
