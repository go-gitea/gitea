// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestBranchRuleMatch(t *testing.T) {
	kases := []struct {
		Rule          string
		BranchName    string
		ExpectedMatch bool
	}{
		{
			Rule:          "release/*",
			BranchName:    "release/v1.17",
			ExpectedMatch: true,
		},
		{
			Rule:          "release/**/v1.17",
			BranchName:    "release/test/v1.17",
			ExpectedMatch: true,
		},
		{
			Rule:          "release/**/v1.17",
			BranchName:    "release/test/1/v1.17",
			ExpectedMatch: true,
		},
		{
			Rule:          "release/*/v1.17",
			BranchName:    "release/test/1/v1.17",
			ExpectedMatch: false,
		},
		{
			Rule:          "release/v*",
			BranchName:    "release/v1.16",
			ExpectedMatch: true,
		},
		{
			Rule:          "*",
			BranchName:    "release/v1.16",
			ExpectedMatch: false,
		},
		{
			Rule:          "**",
			BranchName:    "release/v1.16",
			ExpectedMatch: true,
		},
		{
			Rule:          "main",
			BranchName:    "main",
			ExpectedMatch: true,
		},
		{
			Rule:          "master",
			BranchName:    "main",
			ExpectedMatch: false,
		},
	}

	for _, kase := range kases {
		pb := ProtectedBranch{RuleName: kase.Rule}
		var should, infact string
		if !kase.ExpectedMatch {
			should = " not"
		} else {
			infact = " not"
		}
		assert.EqualValues(t, kase.ExpectedMatch, pb.Match(kase.BranchName),
			"%s should%s match %s but it is%s", kase.BranchName, should, kase.Rule, infact,
		)
	}
}

func TestUpdateProtectBranchPriorities(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	// Create some test protected branches with initial priorities
	protectedBranches := []*ProtectedBranch{
		{
			RepoID:   repo.ID,
			RuleName: "master",
			Priority: 1,
		},
		{
			RepoID:   repo.ID,
			RuleName: "develop",
			Priority: 2,
		},
		{
			RepoID:   repo.ID,
			RuleName: "feature/*",
			Priority: 3,
		},
	}

	for _, pb := range protectedBranches {
		_, err := db.GetEngine(db.DefaultContext).Insert(pb)
		assert.NoError(t, err)
	}

	// Test updating priorities
	newPriorities := []int64{protectedBranches[2].ID, protectedBranches[0].ID, protectedBranches[1].ID}
	err := UpdateProtectBranchPriorities(db.DefaultContext, repo, newPriorities)
	assert.NoError(t, err)

	// Verify new priorities
	pbs, err := FindRepoProtectedBranchRules(db.DefaultContext, repo.ID)
	assert.NoError(t, err)

	expectedPriorities := map[string]int64{
		"feature/*": 1,
		"master":    2,
		"develop":   3,
	}

	for _, pb := range pbs {
		assert.Equal(t, expectedPriorities[pb.RuleName], pb.Priority)
	}
}

func TestNewProtectBranchPriority(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	err := UpdateProtectBranch(db.DefaultContext, repo, &ProtectedBranch{
		RepoID:   repo.ID,
		RuleName: "branch-1",
		Priority: 1,
	}, WhitelistOptions{})
	assert.NoError(t, err)

	newPB := &ProtectedBranch{
		RepoID:   repo.ID,
		RuleName: "branch-2",
		// Priority intentionally omitted
	}

	err = UpdateProtectBranch(db.DefaultContext, repo, newPB, WhitelistOptions{})
	assert.NoError(t, err)

	savedPB2, err := GetFirstMatchProtectedBranchRule(db.DefaultContext, repo.ID, "branch-2")
	assert.NoError(t, err)
	assert.Equal(t, int64(2), savedPB2.Priority)
}
