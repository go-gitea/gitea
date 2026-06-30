// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

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
		assert.Equal(t, kase.ExpectedMatch, pb.Match(kase.BranchName),
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
		_, err := db.GetEngine(t.Context()).Insert(pb)
		assert.NoError(t, err)
	}

	// Test updating priorities
	newPriorities := []int64{protectedBranches[2].ID, protectedBranches[0].ID, protectedBranches[1].ID}
	err := UpdateProtectBranchPriorities(t.Context(), repo, newPriorities)
	assert.NoError(t, err)

	// Verify new priorities
	pbs, err := FindRepoProtectedBranchRules(t.Context(), repo.ID)
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

	err := UpdateProtectBranch(t.Context(), repo, &ProtectedBranch{
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

	err = UpdateProtectBranch(t.Context(), repo, newPB, WhitelistOptions{})
	assert.NoError(t, err)

	savedPB2, err := GetFirstMatchProtectedBranchRule(t.Context(), repo.ID, "branch-2")
	assert.NoError(t, err)
	assert.Equal(t, int64(2), savedPB2.Priority)
}

func TestCanActionsUserPush(t *testing.T) {
	// The Actions bot is a virtual user that cannot be added to a push allowlist, so its push
	// permission must be derived from the token's computed permission instead of user lookup.
	codeUnits := []*repo_model.RepoUnit{{Type: unit.TypeCode}}

	writePerm := access_model.Permission{}
	writePerm.SetUnitsWithDefaultAccessMode(codeUnits, perm.AccessModeWrite)

	readPerm := access_model.Permission{}
	readPerm.SetUnitsWithDefaultAccessMode(codeUnits, perm.AccessModeRead)

	t.Run("Push", func(t *testing.T) {
		// No whitelist enforced + code-write: allowed, just like a normal write user.
		assert.True(t, (&ProtectedBranch{CanPush: true, EnableWhitelist: false}).CanActionsUserPush(writePerm))
		// No whitelist enforced but no code-write: denied.
		assert.False(t, (&ProtectedBranch{CanPush: true, EnableWhitelist: false}).CanActionsUserPush(readPerm))
		// Whitelist enforced: the bot can never be on it, so denied even with code-write.
		assert.False(t, (&ProtectedBranch{CanPush: true, EnableWhitelist: true}).CanActionsUserPush(writePerm))
		// Push disabled entirely: denied.
		assert.False(t, (&ProtectedBranch{CanPush: false, EnableWhitelist: false}).CanActionsUserPush(writePerm))
	})

	t.Run("ForcePush", func(t *testing.T) {
		// Force-push allowed when force-push is enabled without allowlist and regular push is allowed.
		assert.True(t, (&ProtectedBranch{
			CanPush: true, CanForcePush: true, EnableForcePushAllowlist: false,
		}).CanActionsUserForcePush(writePerm))
		// Force-push allowlist enforced: the bot can never be on it, so denied.
		assert.False(t, (&ProtectedBranch{
			CanPush: true, CanForcePush: true, EnableForcePushAllowlist: true,
		}).CanActionsUserForcePush(writePerm))
		// Force-push disabled: denied.
		assert.False(t, (&ProtectedBranch{
			CanPush: true, CanForcePush: false, EnableForcePushAllowlist: false,
		}).CanActionsUserForcePush(writePerm))
		// Regular push not allowed (e.g. push whitelist enforced): force-push also denied.
		assert.False(t, (&ProtectedBranch{
			CanPush: true, EnableWhitelist: true, CanForcePush: true, EnableForcePushAllowlist: false,
		}).CanActionsUserForcePush(writePerm))
	})
}

func TestCanBypassBranchProtection(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}) // not in team 1
	teamMember := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	pb := &ProtectedBranch{
		EnableBypassAllowlist:  true,
		BypassAllowlistUserIDs: []int64{user.ID},
	}

	testBypass := func(t *testing.T, expected bool, pb *ProtectedBranch, doer *user_model.User, isAdmin bool) {
		assert.Equal(t, expected, CanBypassBranchProtection(t.Context(), pb, doer, isAdmin))
	}
	// User bypasses via explicit allowlist.
	testBypass(t, true, pb, user, false)

	// Non-admin cannot bypass when allowlist is disabled.
	pb.EnableBypassAllowlist = false
	testBypass(t, false, pb, user, false)

	// Repo admin can bypass independently of allowlist when not blocked.
	testBypass(t, true, pb, user, true)

	// Admin override block still allows bypass for allowlisted users.
	pb.EnableBypassAllowlist = true
	pb.BlockAdminMergeOverride = true
	testBypass(t, true, pb, user, false)

	// admin cannot bypass without allowlist membership.
	pb.BypassAllowlistUserIDs = nil
	testBypass(t, false, pb, user, true)

	// admin can bypass when allowlisted.
	pb.BypassAllowlistUserIDs = []int64{user.ID}
	testBypass(t, true, pb, user, true)

	// User bypasses via team allowlist membership.
	pb.EnableBypassAllowlist = true
	pb.BlockAdminMergeOverride = false
	pb.BypassAllowlistUserIDs = nil
	pb.BypassAllowlistTeamIDs = []int64{1} // team 1 contains user 2 in test fixtures
	testBypass(t, true, pb, teamMember, false)

	// User does not bypass when not in allowlisted teams.
	testBypass(t, false, pb, user, false)
}
