// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/markup"

	"github.com/stretchr/testify/assert"
)

func TestGetRepositoryCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	count, err1 := GetRepositoryCount(db.DefaultContext, 10)
	privateCount, err2 := GetPrivateRepositoryCount(&user_model.User{ID: int64(10)})
	publicCount, err3 := GetPublicRepositoryCount(&user_model.User{ID: int64(10)})
	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NoError(t, err3)
	assert.Equal(t, int64(3), count)
	assert.Equal(t, privateCount+publicCount, count)
}

func TestGetPublicRepositoryCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	count, err := GetPublicRepositoryCount(&user_model.User{ID: int64(10)})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestGetPrivateRepositoryCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	count, err := GetPrivateRepositoryCount(&user_model.User{ID: int64(10)})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestMetas(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := &Repository{Name: "testRepo"}
	repo.Owner = &user_model.User{Name: "testOwner"}
	repo.OwnerName = repo.Owner.Name

	repo.Units = nil

	metas := repo.ComposeMetas()
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
		repo.RenderingMetas = nil
		metas := repo.ComposeMetas()
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

	repo, err := GetRepositoryByID(3)
	assert.NoError(t, err)

	metas = repo.ComposeMetas()
	assert.Contains(t, metas, "org")
	assert.Contains(t, metas, "teams")
	assert.Equal(t, "user3", metas["org"])
	assert.Equal(t, ",owners,team1,", metas["teams"])
}

func TestRepoAPIURL(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 10}).(*Repository)

	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user12/repo10", repo.APIURL())
}
