// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/release"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateNewTagProtected(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	t.Run("Code", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		err := release.CreateNewTag(git.DefaultContext, owner, repo, "master", "t-first", "first tag")
		assert.NoError(t, err)

		err = release.CreateNewTag(git.DefaultContext, owner, repo, "master", "v-2", "second tag")
		assert.Error(t, err)
		assert.True(t, release.IsErrProtectedTagName(err))

		err = release.CreateNewTag(git.DefaultContext, owner, repo, "master", "v-1.1", "third tag")
		assert.NoError(t, err)
	})

	t.Run("Git", func(t *testing.T) {
		onGiteaRun(t, func(t *testing.T, u *url.URL) {
			httpContext := NewAPITestContext(t, owner.Name, repo.Name)

			dstPath := t.TempDir()

			u.Path = httpContext.GitPath()
			u.User = url.UserPassword(owner.Name, userPassword)

			doGitClone(dstPath, u)(t)

			_, _, err := git.NewCommand("tag", "v-2").RunStdString(git.DefaultContext, &git.RunOpts{Dir: dstPath})
			assert.NoError(t, err)

			_, _, err = git.NewCommand("push", "--tags").RunStdString(git.DefaultContext, &git.RunOpts{Dir: dstPath})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "Tag v-2 is protected")
		})
	})

	t.Run("GitTagForce", func(t *testing.T) {
		onGiteaRun(t, func(t *testing.T, u *url.URL) {
			httpContext := NewAPITestContext(t, owner.Name, repo.Name)

			dstPath := t.TempDir()

			u.Path = httpContext.GitPath()
			u.User = url.UserPassword(owner.Name, userPassword)

			doGitClone(dstPath, u)(t)

			_, _, err := git.NewCommand("tag", "v-1.1", "-m", "force update", "--force").RunStdString(git.DefaultContext, &git.RunOpts{Dir: dstPath})
			require.NoError(t, err)

			_, _, err = git.NewCommand("push", "--tags").RunStdString(git.DefaultContext, &git.RunOpts{Dir: dstPath})
			require.NoError(t, err)

			_, _, err = git.NewCommand("tag", "v-1.1", "-m", "force update v2", "--force").RunStdString(git.DefaultContext, &git.RunOpts{Dir: dstPath})
			require.NoError(t, err)

			_, _, err = git.NewCommand("push", "--tags").RunStdString(git.DefaultContext, &git.RunOpts{Dir: dstPath})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "the tag already exists in the remote")

			_, _, err = git.NewCommand("push", "--tags", "--force").RunStdString(git.DefaultContext, &git.RunOpts{Dir: dstPath})
			require.NoError(t, err)
			req := NewRequestf(t, "GET", "/%s/releases/tag/v-1.1", repo.FullName())
			resp := MakeRequest(t, req, http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)
			tagsTab := htmlDoc.Find(".release-list-title")
			assert.Contains(t, tagsTab.Text(), "force update v2")
		})
	})

	// Cleanup
	releases, err := db.Find[repo_model.Release](db.DefaultContext, repo_model.FindReleasesOptions{
		IncludeTags: true,
		TagNames:    []string{"v-1", "v-1.1"},
		RepoID:      repo.ID,
	})
	assert.NoError(t, err)

	for _, release := range releases {
		_, err = db.DeleteByID[repo_model.Release](db.DefaultContext, release.ID)
		assert.NoError(t, err)
	}

	protectedTags, err := git_model.GetProtectedTags(db.DefaultContext, repo.ID)
	assert.NoError(t, err)

	for _, protectedTag := range protectedTags {
		err = git_model.DeleteProtectedTag(db.DefaultContext, protectedTag)
		assert.NoError(t, err)
	}
}

func TestRepushTag(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
		session := loginUser(t, owner.LowerName)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		httpContext := NewAPITestContext(t, owner.Name, repo.Name)

		dstPath := t.TempDir()

		u.Path = httpContext.GitPath()
		u.User = url.UserPassword(owner.Name, userPassword)

		doGitClone(dstPath, u)(t)

		// create and push a tag
		_, _, err := git.NewCommand("tag", "v2.0").RunStdString(git.DefaultContext, &git.RunOpts{Dir: dstPath})
		assert.NoError(t, err)
		_, _, err = git.NewCommand("push", "origin", "--tags", "v2.0").RunStdString(git.DefaultContext, &git.RunOpts{Dir: dstPath})
		assert.NoError(t, err)
		// create a release for the tag
		createdRelease := createNewReleaseUsingAPI(t, token, owner, repo, "v2.0", "", "Release of v2.0", "desc")
		assert.False(t, createdRelease.IsDraft)
		// delete the tag
		_, _, err = git.NewCommand("push", "origin", "--delete", "v2.0").RunStdString(git.DefaultContext, &git.RunOpts{Dir: dstPath})
		assert.NoError(t, err)
		// query the release by API and it should be a draft
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/releases/tags/%s", owner.Name, repo.Name, "v2.0"))
		resp := MakeRequest(t, req, http.StatusOK)
		var respRelease *api.Release
		DecodeJSON(t, resp, &respRelease)
		assert.True(t, respRelease.IsDraft)
		// re-push the tag
		_, _, err = git.NewCommand("push", "origin", "--tags", "v2.0").RunStdString(git.DefaultContext, &git.RunOpts{Dir: dstPath})
		assert.NoError(t, err)
		// query the release by API and it should not be a draft
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/releases/tags/%s", owner.Name, repo.Name, "v2.0"))
		resp = MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &respRelease)
		assert.False(t, respRelease.IsDraft)
	})
}
