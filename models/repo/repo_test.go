// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"code.gitea.io/gitea/models/db"
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
	countRepospts        = CountRepositoryOptions{OwnerID: 10}
	countReposptsPublic  = CountRepositoryOptions{OwnerID: 10, Private: optional.Some(false)}
	countReposptsPrivate = CountRepositoryOptions{OwnerID: 10, Private: optional.Some(true)}
)

func TestGetRepositoryCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ctx := db.DefaultContext
	count, err1 := CountRepositories(ctx, countRepospts)
	privateCount, err2 := CountRepositories(ctx, countReposptsPrivate)
	publicCount, err3 := CountRepositories(ctx, countReposptsPublic)
	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NoError(t, err3)
	assert.Equal(t, int64(3), count)
	assert.Equal(t, privateCount+publicCount, count)
}

func TestGetPublicRepositoryCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	count, err := CountRepositories(db.DefaultContext, countReposptsPublic)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestGetPrivateRepositoryCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	count, err := CountRepositories(db.DefaultContext, countReposptsPrivate)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestRepoAPIURL(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 10})

	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user12/repo10", repo.APIURL())
}

func TestWatchRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 3})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	assert.NoError(t, WatchRepo(db.DefaultContext, user, repo, true))
	unittest.AssertExistsAndLoadBean(t, &Watch{RepoID: repo.ID, UserID: user.ID})
	unittest.CheckConsistencyFor(t, &Repository{ID: repo.ID})

	assert.NoError(t, WatchRepo(db.DefaultContext, user, repo, false))
	unittest.AssertNotExistsBean(t, &Watch{RepoID: repo.ID, UserID: user.ID})
	unittest.CheckConsistencyFor(t, &Repository{ID: repo.ID})
}

func TestMetas(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := &Repository{Name: "testRepo"}
	repo.Owner = &user_model.User{Name: "testOwner"}
	repo.OwnerName = repo.Owner.Name

	repo.Units = nil

	metas := repo.ComposeMetas(db.DefaultContext)
	assert.Equal(t, "testRepo", metas["repo"])
	assert.Equal(t, "testOwner", metas["user"])

	externalTracker := RepoUnit{
		Type: unit.TypeExternalTracker,
		Config: &ExternalTrackerConfig{
			ExternalTrackerFormat: "https://someurl.com/{user}/{repo}/{issue}",
		},
	}

	testSuccess := func(expectedStyle string) {
		repo.Units = []*RepoUnit{&externalTracker}
		repo.commonRenderingMetas = nil
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

	repo, err := GetRepositoryByID(db.DefaultContext, 3)
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
		repo, err := GetRepositoryByURL(db.DefaultContext, "something")

		assert.Nil(t, repo)
		assert.Error(t, err)
	})

	t.Run("ValidHttpURL", func(t *testing.T) {
		test := func(t *testing.T, url string) {
			repo, err := GetRepositoryByURL(db.DefaultContext, url)

			assert.NotNil(t, repo)
			assert.NoError(t, err)

			assert.Equal(t, int64(2), repo.ID)
			assert.Equal(t, int64(2), repo.OwnerID)
		}

		test(t, "https://try.gitea.io/user2/repo2")
		test(t, "https://try.gitea.io/user2/repo2.git")
	})

	t.Run("ValidGitSshURL", func(t *testing.T) {
		test := func(t *testing.T, url string) {
			repo, err := GetRepositoryByURL(db.DefaultContext, url)

			assert.NotNil(t, repo)
			assert.NoError(t, err)

			assert.Equal(t, int64(2), repo.ID)
			assert.Equal(t, int64(2), repo.OwnerID)
		}

		test(t, "git+ssh://sshuser@try.gitea.io/user2/repo2")
		test(t, "git+ssh://sshuser@try.gitea.io/user2/repo2.git")

		test(t, "git+ssh://try.gitea.io/user2/repo2")
		test(t, "git+ssh://try.gitea.io/user2/repo2.git")
	})

	t.Run("ValidImplicitSshURL", func(t *testing.T) {
		test := func(t *testing.T, url string) {
			repo, err := GetRepositoryByURL(db.DefaultContext, url)

			assert.NotNil(t, repo)
			assert.NoError(t, err)

			assert.Equal(t, int64(2), repo.ID)
			assert.Equal(t, int64(2), repo.OwnerID)
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
	assert.Equal(t, "git@domain:user/repo.git", ComposeSSHCloneURL("user", "repo"))
	setting.Repository.UseCompatSSHURI = true
	assert.Equal(t, "ssh://git@domain/user/repo.git", ComposeSSHCloneURL("user", "repo"))
	// test SSH_DOMAIN while use non-standard SSH port
	setting.SSH.Port = 123
	setting.Repository.UseCompatSSHURI = false
	assert.Equal(t, "ssh://git@domain:123/user/repo.git", ComposeSSHCloneURL("user", "repo"))
	setting.Repository.UseCompatSSHURI = true
	assert.Equal(t, "ssh://git@domain:123/user/repo.git", ComposeSSHCloneURL("user", "repo"))

	// test IPv6 SSH_DOMAIN
	setting.Repository.UseCompatSSHURI = false
	setting.SSH.Domain = "::1"
	setting.SSH.Port = 22
	assert.Equal(t, "git@[::1]:user/repo.git", ComposeSSHCloneURL("user", "repo"))
	setting.SSH.Port = 123
	assert.Equal(t, "ssh://git@[::1]:123/user/repo.git", ComposeSSHCloneURL("user", "repo"))
}
