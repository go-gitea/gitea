// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestAddDeletedBranch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	firstBranch := unittest.AssertExistsAndLoadBean(t, &git_model.Branch{ID: 1})

	assert.True(t, firstBranch.IsDeleted)
	assert.NoError(t, git_model.AddDeletedBranch(db.DefaultContext, repo.ID, firstBranch.Name, firstBranch.DeletedByID))
	assert.NoError(t, git_model.AddDeletedBranch(db.DefaultContext, repo.ID, "branch2", int64(1)))

	secondBranch := unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: repo.ID, Name: "branch2"})
	assert.True(t, secondBranch.IsDeleted)

	commit := &git.Commit{
		ID:            git.MustIDFromString(secondBranch.CommitID),
		CommitMessage: secondBranch.CommitMessage,
		Committer: &git.Signature{
			When: secondBranch.CommitTime.AsLocalTime(),
		},
	}

	err := git_model.UpdateBranch(db.DefaultContext, repo.ID, secondBranch.PusherID, secondBranch.Name, commit)
	assert.NoError(t, err)
}

func TestGetDeletedBranches(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	branches, err := git_model.FindBranches(db.DefaultContext, git_model.FindBranchOptions{
		ListOptions: db.ListOptions{
			ListAll: true,
		},
		RepoID:          repo.ID,
		IsDeletedBranch: util.OptionalBoolTrue,
	})
	assert.NoError(t, err)
	assert.Len(t, branches, 2)
}

func TestGetDeletedBranch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	firstBranch := unittest.AssertExistsAndLoadBean(t, &git_model.Branch{ID: 1})

	assert.NotNil(t, getDeletedBranch(t, firstBranch))
}

func TestDeletedBranchLoadUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	firstBranch := unittest.AssertExistsAndLoadBean(t, &git_model.Branch{ID: 1})
	secondBranch := unittest.AssertExistsAndLoadBean(t, &git_model.Branch{ID: 2})

	branch := getDeletedBranch(t, firstBranch)
	assert.Nil(t, branch.DeletedBy)
	branch.LoadDeletedBy(db.DefaultContext)
	assert.NotNil(t, branch.DeletedBy)
	assert.Equal(t, "user1", branch.DeletedBy.Name)

	branch = getDeletedBranch(t, secondBranch)
	assert.Nil(t, branch.DeletedBy)
	branch.LoadDeletedBy(db.DefaultContext)
	assert.NotNil(t, branch.DeletedBy)
	assert.Equal(t, "Ghost", branch.DeletedBy.Name)
}

func TestRemoveDeletedBranch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	firstBranch := unittest.AssertExistsAndLoadBean(t, &git_model.Branch{ID: 1})

	err := git_model.RemoveDeletedBranchByID(db.DefaultContext, repo.ID, 1)
	assert.NoError(t, err)
	unittest.AssertNotExistsBean(t, firstBranch)
	unittest.AssertExistsAndLoadBean(t, &git_model.Branch{ID: 2})
}

func getDeletedBranch(t *testing.T, branch *git_model.Branch) *git_model.Branch {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	deletedBranch, err := git_model.GetDeletedBranchByID(db.DefaultContext, repo.ID, branch.ID)
	assert.NoError(t, err)
	assert.Equal(t, branch.ID, deletedBranch.ID)
	assert.Equal(t, branch.Name, deletedBranch.Name)
	assert.Equal(t, branch.CommitID, deletedBranch.CommitID)
	assert.Equal(t, branch.DeletedByID, deletedBranch.DeletedByID)

	return deletedBranch
}

func TestFindRenamedBranch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	branch, exist, err := git_model.FindRenamedBranch(db.DefaultContext, 1, "dev")
	assert.NoError(t, err)
	assert.True(t, exist)
	assert.Equal(t, "master", branch.To)

	_, exist, err = git_model.FindRenamedBranch(db.DefaultContext, 1, "unknow")
	assert.NoError(t, err)
	assert.False(t, exist)
}

func TestRenameBranch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	_isDefault := false

	ctx, committer, err := db.TxContext(db.DefaultContext)
	defer committer.Close()
	assert.NoError(t, err)
	assert.NoError(t, git_model.UpdateProtectBranch(ctx, repo1, &git_model.ProtectedBranch{
		RepoID:   repo1.ID,
		RuleName: "master",
	}, git_model.WhitelistOptions{}))
	assert.NoError(t, committer.Commit())

	assert.NoError(t, git_model.RenameBranch(db.DefaultContext, repo1, "master", "main", func(isDefault bool) error {
		_isDefault = isDefault
		return nil
	}))

	assert.True(t, _isDefault)
	repo1 = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.Equal(t, "main", repo1.DefaultBranch)

	pull := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1}) // merged
	assert.Equal(t, "master", pull.BaseBranch)

	pull = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2}) // open
	assert.Equal(t, "main", pull.BaseBranch)

	renamedBranch := unittest.AssertExistsAndLoadBean(t, &git_model.RenamedBranch{ID: 2})
	assert.Equal(t, "master", renamedBranch.From)
	assert.Equal(t, "main", renamedBranch.To)
	assert.Equal(t, int64(1), renamedBranch.RepoID)

	unittest.AssertExistsAndLoadBean(t, &git_model.ProtectedBranch{
		RepoID:   repo1.ID,
		RuleName: "main",
	})
}

func TestOnlyGetDeletedBranchOnCorrectRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Get deletedBranch with ID of 1 on repo with ID 2.
	// This should return a nil branch as this deleted branch
	// is actually on repo with ID 1.
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})

	deletedBranch, err := git_model.GetDeletedBranchByID(db.DefaultContext, repo2.ID, 1)

	// Expect error, and the returned branch is nil.
	assert.Error(t, err)
	assert.Nil(t, deletedBranch)

	// Now get the deletedBranch with ID of 1 on repo with ID 1.
	// This should return the deletedBranch.
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	deletedBranch, err = git_model.GetDeletedBranchByID(db.DefaultContext, repo1.ID, 1)

	// Expect no error, and the returned branch to be not nil.
	assert.NoError(t, err)
	assert.NotNil(t, deletedBranch)
}
