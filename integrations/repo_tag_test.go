// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/url"
	"os"
	"testing"

	"code.gitea.io/gitea/models"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/release"

	"github.com/stretchr/testify/assert"
)

func TestCreateNewTagProtected(t *testing.T) {
	defer prepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID}).(*user_model.User)

	t.Run("API", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		err := release.CreateNewTag(owner, repo, "master", "v-1", "first tag")
		assert.NoError(t, err)

		err = models.InsertProtectedTag(&models.ProtectedTag{
			RepoID:      repo.ID,
			NamePattern: "v-*",
		})
		assert.NoError(t, err)
		err = models.InsertProtectedTag(&models.ProtectedTag{
			RepoID:           repo.ID,
			NamePattern:      "v-1.1",
			AllowlistUserIDs: []int64{repo.OwnerID},
		})
		assert.NoError(t, err)

		err = release.CreateNewTag(owner, repo, "master", "v-2", "second tag")
		assert.Error(t, err)
		assert.True(t, models.IsErrProtectedTagName(err))

		err = release.CreateNewTag(owner, repo, "master", "v-1.1", "third tag")
		assert.NoError(t, err)
	})

	t.Run("Git", func(t *testing.T) {
		onGiteaRun(t, func(t *testing.T, u *url.URL) {
			username := "user2"
			httpContext := NewAPITestContext(t, username, "repo1")

			dstPath, err := os.MkdirTemp("", httpContext.Reponame)
			assert.NoError(t, err)
			defer util.RemoveAll(dstPath)

			u.Path = httpContext.GitPath()
			u.User = url.UserPassword(username, userPassword)

			doGitClone(dstPath, u)(t)

			_, err = git.NewCommand("tag", "v-2").RunInDir(dstPath)
			assert.NoError(t, err)

			_, err = git.NewCommand("push", "--tags").RunInDir(dstPath)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "Tag v-2 is protected")
		})
	})
}
