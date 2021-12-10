// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"image"
	"image/png"
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/markup"

	"github.com/stretchr/testify/assert"
)

func TestMetas(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := &repo_model.Repository{Name: "testRepo"}
	repo.Owner = &user_model.User{Name: "testOwner"}
	repo.OwnerName = repo.Owner.Name

	repo.Units = nil

	metas := repo.ComposeMetas()
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

	repo, err := repo_model.GetRepositoryByID(3)
	assert.NoError(t, err)

	metas = repo.ComposeMetas()
	assert.Contains(t, metas, "org")
	assert.Contains(t, metas, "teams")
	assert.Equal(t, "user3", metas["org"])
	assert.Equal(t, ",owners,team1,", metas["teams"])
}

func TestUpdateRepositoryVisibilityChanged(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Get sample repo and change visibility
	repo, err := repo_model.GetRepositoryByID(9)
	assert.NoError(t, err)
	repo.IsPrivate = true

	// Update it
	err = UpdateRepository(repo, true)
	assert.NoError(t, err)

	// Check visibility of action has become private
	act := Action{}
	_, err = db.GetEngine(db.DefaultContext).ID(3).Get(&act)

	assert.NoError(t, err)
	assert.True(t, act.IsPrivate)
}

func TestGetUserFork(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// User13 has repo 11 forked from repo10
	repo, err := repo_model.GetRepositoryByID(10)
	assert.NoError(t, err)
	assert.NotNil(t, repo)
	repo, err = GetUserFork(repo.ID, 13)
	assert.NoError(t, err)
	assert.NotNil(t, repo)

	repo, err = repo_model.GetRepositoryByID(9)
	assert.NoError(t, err)
	assert.NotNil(t, repo)
	repo, err = GetUserFork(repo.ID, 13)
	assert.NoError(t, err)
	assert.Nil(t, repo)
}

func TestRepoAPIURL(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10}).(*repo_model.Repository)

	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user12/repo10", repo.APIURL())
}

func TestUploadAvatar(t *testing.T) {
	// Generate image
	myImage := image.NewRGBA(image.Rect(0, 0, 1, 1))
	var buff bytes.Buffer
	png.Encode(&buff, myImage)

	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10}).(*repo_model.Repository)

	err := UploadRepoAvatar(repo, buff.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%d-%x", 10, md5.Sum(buff.Bytes())), repo.Avatar)
}

func TestUploadBigAvatar(t *testing.T) {
	// Generate BIG image
	myImage := image.NewRGBA(image.Rect(0, 0, 5000, 1))
	var buff bytes.Buffer
	png.Encode(&buff, myImage)

	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10}).(*repo_model.Repository)

	err := UploadRepoAvatar(repo, buff.Bytes())
	assert.Error(t, err)
}

func TestDeleteAvatar(t *testing.T) {
	// Generate image
	myImage := image.NewRGBA(image.Rect(0, 0, 1, 1))
	var buff bytes.Buffer
	png.Encode(&buff, myImage)

	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10}).(*repo_model.Repository)

	err := UploadRepoAvatar(repo, buff.Bytes())
	assert.NoError(t, err)

	err = DeleteRepoAvatar(repo)
	assert.NoError(t, err)

	assert.Equal(t, "", repo.Avatar)
}

func TestDoctorUserStarNum(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	assert.NoError(t, DoctorUserStarNum())
}

func TestRepoGetReviewers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// test public repo
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)

	reviewers, err := GetReviewers(repo1, 2, 2)
	assert.NoError(t, err)
	assert.Len(t, reviewers, 4)

	// test private repo
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2}).(*repo_model.Repository)
	reviewers, err = GetReviewers(repo2, 2, 2)
	assert.NoError(t, err)
	assert.Empty(t, reviewers)
}

func TestRepoGetReviewerTeams(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2}).(*repo_model.Repository)
	teams, err := GetReviewerTeams(repo2)
	assert.NoError(t, err)
	assert.Empty(t, teams)

	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3}).(*repo_model.Repository)
	teams, err = GetReviewerTeams(repo3)
	assert.NoError(t, err)
	assert.Len(t, teams, 2)
}

func TestLinkedRepository(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	testCases := []struct {
		name             string
		attachID         int64
		expectedRepo     *repo_model.Repository
		expectedUnitType unit.Type
	}{
		{"LinkedIssue", 1, &repo_model.Repository{ID: 1}, unit.TypeIssues},
		{"LinkedComment", 3, &repo_model.Repository{ID: 1}, unit.TypePullRequests},
		{"LinkedRelease", 9, &repo_model.Repository{ID: 1}, unit.TypeReleases},
		{"Notlinked", 10, nil, -1},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			attach, err := repo_model.GetAttachmentByID(tc.attachID)
			assert.NoError(t, err)
			repo, unitType, err := LinkedRepository(attach)
			assert.NoError(t, err)
			if tc.expectedRepo != nil {
				assert.Equal(t, tc.expectedRepo.ID, repo.ID)
			}
			assert.Equal(t, tc.expectedUnitType, unitType)
		})
	}
}
