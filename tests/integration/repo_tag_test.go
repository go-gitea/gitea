// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/url"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/services/release"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestCreateNewTagProtected(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	t.Run("API", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		err := release.CreateNewTag(git.DefaultContext, owner, repo, "master", "v-1", "first tag")
		assert.NoError(t, err)

		err = git_model.InsertProtectedTag(&git_model.ProtectedTag{
			RepoID:      repo.ID,
			NamePattern: "v-*",
		})
		assert.NoError(t, err)
		err = git_model.InsertProtectedTag(&git_model.ProtectedTag{
			RepoID:           repo.ID,
			NamePattern:      "v-1.1",
			AllowlistUserIDs: []int64{repo.OwnerID},
		})
		assert.NoError(t, err)

		err = release.CreateNewTag(git.DefaultContext, owner, repo, "master", "v-2", "second tag")
		assert.Error(t, err)
		assert.True(t, models.IsErrProtectedTagName(err))

		err = release.CreateNewTag(git.DefaultContext, owner, repo, "master", "v-1.1", "third tag")
		assert.NoError(t, err)
	})

	t.Run("Git", func(t *testing.T) {
		onGiteaRun(t, func(t *testing.T, u *url.URL) {
			username := "user2"
			httpContext := NewAPITestContext(t, username, "repo1")

			dstPath := t.TempDir()

			u.Path = httpContext.GitPath()
			u.User = url.UserPassword(username, userPassword)

			doGitClone(dstPath, u)(t)

			_, _, err := git.NewCommand(git.DefaultContext, "tag", "v-2").RunStdString(&git.RunOpts{Dir: dstPath})
			assert.NoError(t, err)

			_, _, err = git.NewCommand(git.DefaultContext, "push", "--tags").RunStdString(&git.RunOpts{Dir: dstPath})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "Tag v-2 is protected")
		})
	})

	// Cleanup
	releases, err := repo_model.GetReleasesByRepoID(db.DefaultContext, repo.ID, repo_model.FindReleasesOptions{
		IncludeTags: true,
		TagNames:    []string{"v-1", "v-1.1"},
	})
	assert.NoError(t, err)

	for _, release := range releases {
		err = repo_model.DeleteReleaseByID(db.DefaultContext, release.ID)
		assert.NoError(t, err)
	}

	protectedTags, err := git_model.GetProtectedTags(db.DefaultContext, repo.ID)
	assert.NoError(t, err)

	for _, protectedTag := range protectedTags {
		err = git_model.DeleteProtectedTag(db.DefaultContext, protectedTag)
		assert.NoError(t, err)
	}
}
