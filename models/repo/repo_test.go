// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

var (
	countRepospts        = repo_model.CountRepositoryOptions{OwnerID: 10}
	countReposptsPublic  = repo_model.CountRepositoryOptions{OwnerID: 10, Private: optional.Some(false)}
	countReposptsPrivate = repo_model.CountRepositoryOptions{OwnerID: 10, Private: optional.Some(true)}
)

func TestGetRepositoryCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ctx := db.DefaultContext
	count, err1 := repo_model.CountRepositories(ctx, countRepospts)
	privateCount, err2 := repo_model.CountRepositories(ctx, countReposptsPrivate)
	publicCount, err3 := repo_model.CountRepositories(ctx, countReposptsPublic)
	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NoError(t, err3)
	assert.Equal(t, int64(3), count)
	assert.Equal(t, privateCount+publicCount, count)
}

func TestGetPublicRepositoryCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	count, err := repo_model.CountRepositories(db.DefaultContext, countReposptsPublic)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestGetPrivateRepositoryCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	count, err := repo_model.CountRepositories(db.DefaultContext, countReposptsPrivate)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestRepoAPIURL(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})

	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user12/repo10", repo.APIURL())
}

func TestWatchRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	const repoID = 3
	const userID = 2

	assert.NoError(t, repo_model.WatchRepo(db.DefaultContext, userID, repoID, true))
	unittest.AssertExistsAndLoadBean(t, &repo_model.Watch{RepoID: repoID, UserID: userID})
	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: repoID})

	assert.NoError(t, repo_model.WatchRepo(db.DefaultContext, userID, repoID, false))
	unittest.AssertNotExistsBean(t, &repo_model.Watch{RepoID: repoID, UserID: userID})
	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: repoID})
}

func TestMetas(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := &repo_model.Repository{Name: "testRepo"}
	repo.Owner = &user_model.User{Name: "testOwner"}
	repo.OwnerName = repo.Owner.Name

	repo.Units = nil

	metas := repo.ComposeMetas(db.DefaultContext)
	assert.Equal(t, "testRepo", metas["repo"])
	assert.Equal(t, "testOwner", metas["user"])

	externalTracker := repo_model.RepoUnit{
		Type: unit.TypeExternalTracker,
		Config: &repo_model.ExternalTrackerConfig{
			ExternalTrackerFormat: "https://someurl.com/{user}/{repo}/{issue}",
		},
	}

	testSuccess := func(expectedStyle string) {
		repo.Units = []*repo_model.RepoUnit{&externalTracker}
		repo.RenderingMetas = nil
		metas := repo.ComposeMetas(db.DefaultContext)
		assert.Equal(t, expectedStyle, metas["style"])
		assert.Equal(t, "testRepo", metas["repo"])
		assert.Equal(t, "testOwner", metas["user"])
		assert.Equal(t, "https://someurl.com/{user}/{repo}/{issue}", metas["format"])
	}

	testSuccess(markup.IssueNameStyleNumeric)

	externalTracker.ExternalTrackerConfig().ExternalTrackerStyle = markup.IssueNameStyleAlphanumeric
	testSuccess(markup.IssueNameStyleAlphanumeric)

	externalTracker.ExternalTrackerConfig().ExternalTrackerStyle = markup.IssueNameStyleNumeric
	testSuccess(markup.IssueNameStyleNumeric)

	externalTracker.ExternalTrackerConfig().ExternalTrackerStyle = markup.IssueNameStyleRegexp
	testSuccess(markup.IssueNameStyleRegexp)

	repo, err := repo_model.GetRepositoryByID(db.DefaultContext, 3)
	assert.NoError(t, err)

	metas = repo.ComposeMetas(db.DefaultContext)
	assert.Contains(t, metas, "org")
	assert.Contains(t, metas, "teams")
	assert.Equal(t, "org3", metas["org"])
	assert.Equal(t, ",owners,team1,", metas["teams"])
}

func TestGetRepositoryByURL(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	t.Run("InvalidPath", func(t *testing.T) {
		repo, err := repo_model.GetRepositoryByURL(db.DefaultContext, "something")

		assert.Nil(t, repo)
		assert.Error(t, err)
	})

	t.Run("ValidHttpURL", func(t *testing.T) {
		test := func(t *testing.T, url string) {
			repo, err := repo_model.GetRepositoryByURL(db.DefaultContext, url)

			assert.NotNil(t, repo)
			assert.NoError(t, err)

			assert.Equal(t, repo.ID, int64(2))
			assert.Equal(t, repo.OwnerID, int64(2))
		}

		test(t, "https://try.gitea.io/user2/repo2")
		test(t, "https://try.gitea.io/user2/repo2.git")
	})

	t.Run("ValidGitSshURL", func(t *testing.T) {
		test := func(t *testing.T, url string) {
			repo, err := repo_model.GetRepositoryByURL(db.DefaultContext, url)

			assert.NotNil(t, repo)
			assert.NoError(t, err)

			assert.Equal(t, repo.ID, int64(2))
			assert.Equal(t, repo.OwnerID, int64(2))
		}

		test(t, "git+ssh://sshuser@try.gitea.io/user2/repo2")
		test(t, "git+ssh://sshuser@try.gitea.io/user2/repo2.git")

		test(t, "git+ssh://try.gitea.io/user2/repo2")
		test(t, "git+ssh://try.gitea.io/user2/repo2.git")
	})

	t.Run("ValidImplicitSshURL", func(t *testing.T) {
		test := func(t *testing.T, url string) {
			repo, err := repo_model.GetRepositoryByURL(db.DefaultContext, url)

			assert.NotNil(t, repo)
			assert.NoError(t, err)

			assert.Equal(t, repo.ID, int64(2))
			assert.Equal(t, repo.OwnerID, int64(2))
		}

		test(t, "sshuser@try.gitea.io:user2/repo2")
		test(t, "sshuser@try.gitea.io:user2/repo2.git")

		test(t, "try.gitea.io:user2/repo2")
		test(t, "try.gitea.io:user2/repo2.git")
	})
}

func TestComposeSSHCloneURL(t *testing.T) {
	defer test.MockVariableValue(&setting.SSH, setting.SSH)()
	defer test.MockVariableValue(&setting.Repository, setting.Repository)()

	setting.SSH.User = "git"

	// test SSH_DOMAIN
	setting.SSH.Domain = "domain"
	setting.SSH.Port = 22
	setting.Repository.UseCompatSSHURI = false
	assert.Equal(t, "git@domain:user/repo.git", repo_model.ComposeSSHCloneURL("user", "repo"))
	setting.Repository.UseCompatSSHURI = true
	assert.Equal(t, "ssh://git@domain/user/repo.git", repo_model.ComposeSSHCloneURL("user", "repo"))
	// test SSH_DOMAIN while use non-standard SSH port
	setting.SSH.Port = 123
	setting.Repository.UseCompatSSHURI = false
	assert.Equal(t, "ssh://git@domain:123/user/repo.git", repo_model.ComposeSSHCloneURL("user", "repo"))
	setting.Repository.UseCompatSSHURI = true
	assert.Equal(t, "ssh://git@domain:123/user/repo.git", repo_model.ComposeSSHCloneURL("user", "repo"))

	// test IPv6 SSH_DOMAIN
	setting.Repository.UseCompatSSHURI = false
	setting.SSH.Domain = "::1"
	setting.SSH.Port = 22
	assert.Equal(t, "git@[::1]:user/repo.git", repo_model.ComposeSSHCloneURL("user", "repo"))
	setting.SSH.Port = 123
	assert.Equal(t, "ssh://git@[::1]:123/user/repo.git", repo_model.ComposeSSHCloneURL("user", "repo"))
}
