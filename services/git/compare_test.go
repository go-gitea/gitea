// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/gitrepo"

	"github.com/stretchr/testify/assert"
)

func TestGetCompareCommitIDsWithMergeBase(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pr.LoadBaseRepo(t.Context()))

	gitRepo, err := gitrepo.OpenRepository(t.Context(), pr.BaseRepo)
	assert.NoError(t, err)
	defer gitRepo.Close()

	headCommit, err := gitRepo.GetBranchCommit(pr.HeadBranch)
	assert.NoError(t, err)

	baseCommit, err := gitRepo.GetBranchCommit(pr.BaseBranch)
	assert.NoError(t, err)

	commitIDs, err := GetCompareCommitIDsWithMergeBase(t.Context(), pr.BaseRepo, pr.BaseBranch, headCommit.ID.String())
	assert.NoError(t, err)
	assert.NotEmpty(t, commitIDs)

	commits, err := gitRepo.CommitsBetweenNotBase(headCommit, baseCommit, pr.BaseBranch)
	assert.NoError(t, err)

	expected := make([]string, 0, len(commits))
	for i := len(commits) - 1; i >= 0; i-- {
		expected = append(expected, commits[i].ID.String())
	}

	assert.Equal(t, expected, commitIDs)
}
