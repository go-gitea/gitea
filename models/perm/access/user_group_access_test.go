// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package access_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	perm_model "code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/usergroup"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupUserGroupAccess builds a three-level group hierarchy, places a user in
// the leaf group, and assigns the root group to the fixture team 2 that owns
// private repo 3.  Returns the repo and the leaf user.
func setupUserGroupAccess(t *testing.T) (*repo_model.Repository, *user_model.User) {
	t.Helper()
	ctx := t.Context()

	// user 8 is not a direct member of team 2.
	leafUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	require.True(t, repo.IsPrivate, "fixture repo 3 must be private")

	root := &usergroup.UserGroup{Name: "acc-root"}
	require.NoError(t, organization.CreateUserGroup(ctx, root))
	mid := &usergroup.UserGroup{Name: "acc-mid", ParentID: root.ID}
	require.NoError(t, organization.CreateUserGroup(ctx, mid))
	leaf := &usergroup.UserGroup{Name: "acc-leaf", ParentID: mid.ID}
	require.NoError(t, organization.CreateUserGroup(ctx, leaf))

	// User is in leaf group; team assigns root group.
	require.NoError(t, organization.AddUserToUserGroup(ctx, leaf.ID, leafUser.ID))
	require.NoError(t, organization.AddUserGroupToTeam(ctx, team.ID, root.ID, team.OrgID))

	require.NoError(t, repo.LoadOwner(ctx))
	require.NoError(t, access_model.RecalculateTeamAccesses(ctx, repo, 0))

	return repo, leafUser
}

// TestRecalculateTeamAccessesIncludesUserGroupMembers verifies that the
// access table is populated for users who join a team via a group hierarchy.
func TestRecalculateTeamAccessesIncludesUserGroupMembers(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	repo, leafUser := setupUserGroupAccess(t)

	access := &access_model.Access{UserID: leafUser.ID, RepoID: repo.ID}
	has, err := db.GetEngine(ctx).Get(access)
	require.NoError(t, err)
	assert.True(t, has, "access table must contain an entry for the user group user")
	assert.GreaterOrEqual(t, access.Mode, perm_model.AccessModeRead)
}

// TestRecalculateUserAccessPreservesUserGroupAccess reproduces the bug where
// RecalculateUserAccess would wipe the access record of a user who is still in
// the team via a user group.
func TestRecalculateUserAccessPreservesUserGroupAccess(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	repo, leafUser := setupUserGroupAccess(t)

	// Simulate the code path triggered by removing a direct team member.
	require.NoError(t, access_model.RecalculateUserAccess(ctx, repo, leafUser.ID))

	access := &access_model.Access{UserID: leafUser.ID, RepoID: repo.ID}
	has, err := db.GetEngine(ctx).Get(access)
	require.NoError(t, err)
	assert.True(t, has, "RecalculateUserAccess must not delete access that comes from a user group")
}

// TestGetIndividualUserRepoPermissionUserGroup checks that per-unit
// permissions are computed from the team for a user group user.
func TestGetIndividualUserRepoPermissionUserGroup(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo, leafUser := setupUserGroupAccess(t)

	perm, err := access_model.GetIndividualUserRepoPermission(t.Context(), repo, leafUser)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, perm.AccessMode, perm_model.AccessModeRead)
}
