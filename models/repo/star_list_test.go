// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestGetStarListByID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	starList, err := repo_model.GetStarListByID(db.DefaultContext, 1)
	assert.NoError(t, err)

	assert.Equal(t, "First List", starList.Name)
	assert.Equal(t, "Description for first List", starList.Description)
	assert.False(t, starList.IsPrivate)

	// Check if ErrStarListNotFound is returned on an not existing ID
	starList, err = repo_model.GetStarListByID(db.DefaultContext, -1)
	assert.True(t, repo_model.IsErrStarListNotFound(err))
	assert.Nil(t, starList)
}

func TestGetStarListByName(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	starList, err := repo_model.GetStarListByName(db.DefaultContext, 1, "First List")
	assert.NoError(t, err)

	assert.Equal(t, int64(1), starList.ID)
	assert.Equal(t, "Description for first List", starList.Description)
	assert.False(t, starList.IsPrivate)

	// Check if ErrStarListNotFound is returned on an not existing Name
	starList, err = repo_model.GetStarListByName(db.DefaultContext, 1, "NotExistingList")
	assert.True(t, repo_model.IsErrStarListNotFound(err))
	assert.Nil(t, starList)
}

func TestGetStarListByUserID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Get only public lists
	starLists, err := repo_model.GetStarListsByUserID(db.DefaultContext, 1, false)
	assert.NoError(t, err)

	assert.Len(t, starLists, 1)

	assert.Equal(t, int64(1), starLists[0].ID)
	assert.Equal(t, "First List", starLists[0].Name)
	assert.Equal(t, "Description for first List", starLists[0].Description)
	assert.False(t, starLists[0].IsPrivate)

	// Get also private lists
	starLists, err = repo_model.GetStarListsByUserID(db.DefaultContext, 1, true)
	assert.NoError(t, err)

	assert.Len(t, starLists, 2)

	assert.Equal(t, int64(1), starLists[0].ID)
	assert.Equal(t, "First List", starLists[0].Name)
	assert.Equal(t, "Description for first List", starLists[0].Description)
	assert.False(t, starLists[0].IsPrivate)

	assert.Equal(t, int64(2), starLists[1].ID)
	assert.Equal(t, "Second List", starLists[1].Name)
	assert.Equal(t, "This is private", starLists[1].Description)
	assert.True(t, starLists[1].IsPrivate)
}

func TestCreateStarList(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Check that you can't create two list with the same name for the same user
	starList, err := repo_model.CreateStarList(db.DefaultContext, 1, "First List", "Test", false)
	assert.True(t, repo_model.IsErrStarListExists(err))
	assert.Nil(t, starList)

	// Now create the star list for real
	starList, err = repo_model.CreateStarList(db.DefaultContext, 1, "My new List", "Test", false)
	assert.NoError(t, err)

	assert.Equal(t, "My new List", starList.Name)
	assert.Equal(t, "Test", starList.Description)
	assert.False(t, starList.IsPrivate)
}

func TestStarListRepositoryCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	starList := unittest.AssertExistsAndLoadBean(t, &repo_model.StarList{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	assert.NoError(t, starList.LoadRepositoryCount(db.DefaultContext, user))

	assert.Equal(t, int64(1), starList.RepositoryCount)
}

func TestStarListAddRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const repoID = 4

	starList := unittest.AssertExistsAndLoadBean(t, &repo_model.StarList{ID: 1})

	assert.NoError(t, starList.AddRepo(db.DefaultContext, repoID))

	assert.NoError(t, starList.LoadRepoIDs(db.DefaultContext))

	assert.True(t, starList.ContainsRepoID(repoID))
}

func TestStarListRemoveRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const repoID = 1

	starList := unittest.AssertExistsAndLoadBean(t, &repo_model.StarList{ID: 1})

	assert.NoError(t, starList.RemoveRepo(db.DefaultContext, repoID))

	assert.NoError(t, starList.LoadRepoIDs(db.DefaultContext))

	assert.False(t, starList.ContainsRepoID(repoID))
}

func TestStarListEditData(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	starList := unittest.AssertExistsAndLoadBean(t, &repo_model.StarList{ID: 1})

	assert.True(t, repo_model.IsErrStarListExists(starList.EditData(db.DefaultContext, "Second List", "New Description", false)))

	assert.NoError(t, starList.EditData(db.DefaultContext, "First List", "New Description", false))

	assert.Equal(t, "First List", starList.Name)
	assert.Equal(t, "New Description", starList.Description)
	assert.False(t, starList.IsPrivate)
}

func TestStarListHasAccess(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	starList := unittest.AssertExistsAndLoadBean(t, &repo_model.StarList{ID: 2})
	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	assert.True(t, starList.HasAccess(user1))
	assert.False(t, starList.HasAccess(user2))

	assert.NoError(t, starList.MustHaveAccess(user1))
	assert.True(t, repo_model.IsErrStarListNotFound(starList.MustHaveAccess(user2)))
}
