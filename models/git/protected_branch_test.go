// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git_test

import (
	"fmt"
	"testing"

	access_model "code.gitea.io/gitea/models/perm/access"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	perm_model "code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
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
		pb := git_model.ProtectedBranch{RuleName: kase.Rule}
		var should, infact string
		if !kase.ExpectedMatch {
			should = " not"
		} else {
			infact = " not"
		}
		assert.EqualValues(t, kase.ExpectedMatch, pb.Match(kase.BranchName),
			fmt.Sprintf("%s should%s match %s but it is%s", kase.BranchName, should, kase.Rule, infact),
		)
	}
}

func TestIsUserOfficialReviewer(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	protectedBranch := &git_model.ProtectedBranch{
		RepoID:                   repo.ID,
		EnableApprovalsWhitelist: false,
	}
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})

	access := &access_model.Access{
		UserID: user.ID,
		RepoID: repo.ID,
		Mode:   perm_model.AccessModeNone,
	}
	assert.NoError(t, db.Insert(db.DefaultContext, access))

	official, err := git_model.IsUserOfficialReviewer(db.DefaultContext, protectedBranch, user)
	assert.NoError(t, err)
	assert.False(t, official)

	access.Mode = perm_model.AccessModeRead
	_, err = db.GetEngine(db.DefaultContext).ID(access.ID).Update(access)
	assert.NoError(t, err)

	official, err = git_model.IsUserOfficialReviewer(db.DefaultContext, protectedBranch, user)
	assert.NoError(t, err)
	assert.True(t, official)
}
