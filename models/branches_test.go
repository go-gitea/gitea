// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var firstBranch = DeletedBranch{
	ID:          1,
	Name:        "foo",
	Commit:      "1213212312313213213132131",
	DeletedByID: int64(1),
}

var secondBranch = DeletedBranch{
	ID:          2,
	Name:        "bar",
	Commit:      "5655464564554545466464655",
	DeletedByID: int64(99),
}

func TestAddDeletedBranch(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	assert.NoError(t, repo.AddDeletedBranch(firstBranch.Name, firstBranch.Commit, firstBranch.DeletedByID))
	assert.Error(t, repo.AddDeletedBranch(firstBranch.Name, firstBranch.Commit, firstBranch.DeletedByID))
	assert.NoError(t, repo.AddDeletedBranch(secondBranch.Name, secondBranch.Commit, secondBranch.DeletedByID))
}
func TestGetDeletedBranches(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	AssertExistsAndLoadBean(t, &DeletedBranch{ID: 1})
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)

	branches, err := repo.GetDeletedBranches()
	assert.NoError(t, err)
	assert.Len(t, branches, 2)
}

func TestGetDeletedBranch(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	assert.NotNil(t, getDeletedBranch(t, firstBranch))
}

func TestDeletedBranchLoadUser(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
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
	assert.NoError(t, PrepareTestDatabase())

	branch := DeletedBranch{ID: 1}
	AssertExistsAndLoadBean(t, &branch)
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)

	err := repo.RemoveDeletedBranch(1)
	assert.NoError(t, err)
	AssertNotExistsBean(t, &branch)
	AssertExistsAndLoadBean(t, &DeletedBranch{ID: 2})
}

func getDeletedBranch(t *testing.T, branch DeletedBranch) *DeletedBranch {
	AssertExistsAndLoadBean(t, &DeletedBranch{ID: 1})
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)

	deletedBranch, err := repo.GetDeletedBranchByID(branch.ID)
	assert.NoError(t, err)
	assert.Equal(t, branch.ID, deletedBranch.ID)
	assert.Equal(t, branch.Name, deletedBranch.Name)
	assert.Equal(t, branch.Commit, deletedBranch.Commit)
	assert.Equal(t, branch.DeletedByID, deletedBranch.DeletedByID)

	return deletedBranch
}
