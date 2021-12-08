// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestAddDeletedBranch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)
	firstBranch := unittest.AssertExistsAndLoadBean(t, &DeletedBranch{ID: 1}).(*DeletedBranch)

	assert.Error(t, AddDeletedBranch(repo.ID, firstBranch.Name, firstBranch.Commit, firstBranch.DeletedByID))
	assert.NoError(t, AddDeletedBranch(repo.ID, "test", "5655464564554545466464656", int64(1)))
}

func TestGetDeletedBranches(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)

	branches, err := GetDeletedBranches(repo.ID)
	assert.NoError(t, err)
	assert.Len(t, branches, 2)
}

func TestGetDeletedBranch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	firstBranch := unittest.AssertExistsAndLoadBean(t, &DeletedBranch{ID: 1}).(*DeletedBranch)

	assert.NotNil(t, getDeletedBranch(t, firstBranch))
}

func TestDeletedBranchLoadUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	firstBranch := unittest.AssertExistsAndLoadBean(t, &DeletedBranch{ID: 1}).(*DeletedBranch)
	secondBranch := unittest.AssertExistsAndLoadBean(t, &DeletedBranch{ID: 2}).(*DeletedBranch)

	branch := getDeletedBranch(t, firstBranch)
	assert.Nil(t, branch.DeletedBy)
	branch.LoadUser()
	assert.NotNil(t, branch.DeletedBy)
	assert.Equal(t, "user1", branch.DeletedBy.Name)

	branch = getDeletedBranch(t, secondBranch)
	assert.Nil(t, branch.DeletedBy)
	branch.LoadUser()
	assert.NotNil(t, branch.DeletedBy)
	assert.Equal(t, "Ghost", branch.DeletedBy.Name)
}

func TestRemoveDeletedBranch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)

	firstBranch := unittest.AssertExistsAndLoadBean(t, &DeletedBranch{ID: 1}).(*DeletedBranch)

	err := RemoveDeletedBranchByID(repo.ID, 1)
	assert.NoError(t, err)
	unittest.AssertNotExistsBean(t, firstBranch)
	unittest.AssertExistsAndLoadBean(t, &DeletedBranch{ID: 2})
}

func getDeletedBranch(t *testing.T, branch *DeletedBranch) *DeletedBranch {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)

	deletedBranch, err := GetDeletedBranchByID(repo.ID, branch.ID)
	assert.NoError(t, err)
	assert.Equal(t, branch.ID, deletedBranch.ID)
	assert.Equal(t, branch.Name, deletedBranch.Name)
	assert.Equal(t, branch.Commit, deletedBranch.Commit)
	assert.Equal(t, branch.DeletedByID, deletedBranch.DeletedByID)

	return deletedBranch
}

func TestFindRenamedBranch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	branch, exist, err := FindRenamedBranch(1, "dev")
	assert.NoError(t, err)
	assert.Equal(t, true, exist)
	assert.Equal(t, "master", branch.To)

	_, exist, err = FindRenamedBranch(1, "unknow")
	assert.NoError(t, err)
	assert.Equal(t, false, exist)
}

func TestRenameBranch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)
	_isDefault := false

	err := UpdateProtectBranch(repo1, &ProtectedBranch{
		RepoID:     repo1.ID,
		BranchName: "master",
	}, WhitelistOptions{})
	assert.NoError(t, err)

	assert.NoError(t, RenameBranch(repo1, "master", "main", func(isDefault bool) error {
		_isDefault = isDefault
		return nil
	}))

	assert.Equal(t, true, _isDefault)
	repo1 = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)
	assert.Equal(t, "main", repo1.DefaultBranch)

	pull := unittest.AssertExistsAndLoadBean(t, &PullRequest{ID: 1}).(*PullRequest) // merged
	assert.Equal(t, "master", pull.BaseBranch)

	pull = unittest.AssertExistsAndLoadBean(t, &PullRequest{ID: 2}).(*PullRequest) // open
	assert.Equal(t, "main", pull.BaseBranch)

	renamedBranch := unittest.AssertExistsAndLoadBean(t, &RenamedBranch{ID: 2}).(*RenamedBranch)
	assert.Equal(t, "master", renamedBranch.From)
	assert.Equal(t, "main", renamedBranch.To)
	assert.Equal(t, int64(1), renamedBranch.RepoID)

	unittest.AssertExistsAndLoadBean(t, &ProtectedBranch{
		RepoID:     repo1.ID,
		BranchName: "main",
	})
}

func TestOnlyGetDeletedBranchOnCorrectRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Get deletedBranch with ID of 1 on repo with ID 2.
	// This should return a nil branch as this deleted branch
	// is actually on repo with ID 1.
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2}).(*repo_model.Repository)

	deletedBranch, err := GetDeletedBranchByID(repo2.ID, 1)

	// Expect no error, and the returned branch is nil.
	assert.NoError(t, err)
	assert.Nil(t, deletedBranch)

	// Now get the deletedBranch with ID of 1 on repo with ID 1.
	// This should return the deletedBranch.
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)

	deletedBranch, err = GetDeletedBranchByID(repo1.ID, 1)

	// Expect no error, and the returned branch to be not nil.
	assert.NoError(t, err)
	assert.NotNil(t, deletedBranch)
}
