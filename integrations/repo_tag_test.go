// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"io/ioutil"
	"net/url"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/release"

	"github.com/stretchr/testify/assert"
)

func TestIsUserAllowedToControlTag(t *testing.T) {
	protectedTags := []*models.ProtectedTag{
		{
			NamePattern:      "*gitea",
			WhitelistUserIDs: []int64{1},
		},
		{
			NamePattern:      "v-*",
			WhitelistUserIDs: []int64{2},
		},
		{
			NamePattern: "release",
		},
	}

	cases := []struct {
		name    string
		userid  int64
		allowed bool
	}{
		{
			name:    "test",
			userid:  1,
			allowed: true,
		},
		{
			name:    "test",
			userid:  3,
			allowed: true,
		},
		{
			name:    "gitea",
			userid:  1,
			allowed: true,
		},
		{
			name:    "gitea",
			userid:  3,
			allowed: false,
		},
		{
			name:    "test-gitea",
			userid:  1,
			allowed: true,
		},
		{
			name:    "test-gitea",
			userid:  3,
			allowed: false,
		},
		{
			name:    "gitea-test",
			userid:  1,
			allowed: true,
		},
		{
			name:    "gitea-test",
			userid:  3,
			allowed: true,
		},
		{
			name:    "v-1",
			userid:  1,
			allowed: false,
		},
		{
			name:    "v-1",
			userid:  2,
			allowed: true,
		},
		{
			name:    "release",
			userid:  1,
			allowed: false,
		},
	}

	for n, c := range cases {
		isAllowed, err := models.IsUserAllowedToControlTag(protectedTags, c.name, c.userid)
		assert.NoError(t, err)
		assert.Equal(t, c.allowed, isAllowed, "case %d: error should match", n)
	}
}

func TestCreateNewTagProtected(t *testing.T) {
	defer prepareTestEnv(t)()

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

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
			WhitelistUserIDs: []int64{repo.OwnerID},
		})
		assert.NoError(t, err)

		err = release.CreateNewTag(owner, repo, "master", "v-2", "second tag")
		assert.Error(t, err)
		assert.True(t, models.IsErrInvalidTagName(err))
		e := err.(models.ErrInvalidTagName)
		assert.True(t, e.Protected)

		err = release.CreateNewTag(owner, repo, "master", "v-1.1", "third tag")
		assert.NoError(t, err)
	})

	t.Run("Git", func(t *testing.T) {
		onGiteaRun(t, func(t *testing.T, u *url.URL) {
			username := "user2"
			httpContext := NewAPITestContext(t, username, "repo1")

			dstPath, err := ioutil.TempDir("", httpContext.Reponame)
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
