// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package action

import (
	"path/filepath"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		GiteaRootPath: filepath.Join("..", "..", ".."),
	})
}

func TestRenameRepoAction(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: user.ID}).(*repo_model.Repository)
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
	unittest.AssertNotExistsBean(t, actionBean)

	NewNotifier().NotifyRenameRepository(user, repo, oldRepoName)

	unittest.AssertExistsAndLoadBean(t, actionBean)
	unittest.CheckConsistencyFor(t, &models.Action{})
}
