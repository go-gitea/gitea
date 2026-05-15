// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package usergroup_test

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/usergroup"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeGroup creates a user group and fails the test on error.
func makeGroup(t *testing.T, name string, parentID int64) *usergroup.UserGroup {
	t.Helper()
	g := &usergroup.UserGroup{Name: name, ParentID: parentID}
	require.NoError(t, usergroup.CreateUserGroup(t.Context(), g))
	return g
}

// containsUser returns true when userID appears in the slice.
func containsUser(members []*user_model.User, userID int64) bool {
	for _, m := range members {
		if m.ID == userID {
			return true
		}
	}
	return false
}

func TestUserGroupParentCycle(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	parent := makeGroup(t, "cycle-parent", 0)
	child := makeGroup(t, "cycle-child", parent.ID)

	parent.ParentID = child.ID
	err := usergroup.UpdateUserGroup(t.Context(), parent)
	require.Error(t, err)
	assert.True(t, usergroup.IsErrUserGroupCircularReference(err))
}

func TestUserGroupNameValidation(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	err := usergroup.CreateUserGroup(t.Context(), &usergroup.UserGroup{Name: "bad/name"})
	require.Error(t, err)
	assert.ErrorIs(t, err, util.ErrInvalidArgument)

	err = usergroup.CreateUserGroup(t.Context(), &usergroup.UserGroup{Name: strings.Repeat("a", 256)})
	require.Error(t, err)
	assert.ErrorIs(t, err, util.ErrInvalidArgument)
}

func TestUserGroupNameUniquenessCaseInsensitive(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	require.NoError(t, usergroup.CreateUserGroup(t.Context(), &usergroup.UserGroup{Name: "Shared Name", Slug: "unique-group"}))
	err := usergroup.CreateUserGroup(t.Context(), &usergroup.UserGroup{Name: "Another Name", Slug: "UNIQUE-GROUP"})
	require.Error(t, err)
	assert.True(t, usergroup.IsErrUserGroupAlreadyExist(err))
}

func TestUserGroupNameCanRepeatWithDifferentSlugs(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	require.NoError(t, usergroup.CreateUserGroup(t.Context(), &usergroup.UserGroup{Name: "Shared Name", Slug: "shared-name-1"}))
	require.NoError(t, usergroup.CreateUserGroup(t.Context(), &usergroup.UserGroup{Name: "Shared Name", Slug: "shared-name-2"}))
}

func TestUserGroupDescriptionPersists(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	longDescription := strings.Repeat("Group description ", 40)
	group := &usergroup.UserGroup{Name: "desc-group", Description: longDescription}
	require.NoError(t, usergroup.CreateUserGroup(ctx, group))

	loaded, err := usergroup.GetUserGroupByID(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(longDescription), loaded.Description)

	loaded.Description = "Updated description"
	require.NoError(t, usergroup.UpdateUserGroup(ctx, loaded))

	updated, err := usergroup.GetUserGroupByID(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated description", updated.Description)
}

// TestGetUserGroupFullPaths checks that nested groups produce the correct
// slash-separated path strings.
func TestGetUserGroupFullPaths(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	root := makeGroup(t, "fp-root", 0)
	mid := makeGroup(t, "fp-mid", root.ID)
	leaf := makeGroup(t, "fp-leaf", mid.ID)

	paths, err := usergroup.GetUserGroupFullPaths(t.Context(), []int64{root.ID, mid.ID, leaf.ID})
	require.NoError(t, err)

	assert.Equal(t, "fp-root", paths[root.ID])
	assert.Equal(t, "fp-root / fp-mid", paths[mid.ID])
	assert.Equal(t, "fp-root / fp-mid / fp-leaf", paths[leaf.ID])
}

// TestGetAvailableUserGroupsForTeam ensures that already-assigned groups are
// excluded from the available list.
func TestGetAvailableUserGroupsForTeam(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})

	g1 := makeGroup(t, "avail-g1", 0)
	g2 := makeGroup(t, "avail-g2", 0)
	g3 := makeGroup(t, "avail-g3", 0)

	// Assign only g1 to the team.
	require.NoError(t, organization.AddUserGroupToTeam(ctx, team.ID, g1.ID, team.OrgID))

	available, err := organization.GetAvailableUserGroupsForTeam(ctx, team.ID)
	require.NoError(t, err)

	ids := make(map[int64]bool, len(available))
	for _, g := range available {
		ids[g.ID] = true
	}
	assert.False(t, ids[g1.ID], "assigned group must not appear in available list")
	assert.True(t, ids[g2.ID], "unassigned group must appear")
	assert.True(t, ids[g3.ID], "unassigned group must appear")
}

func TestIsUserInAnyOrgTeamViaUserGroups(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	g := makeGroup(t, "orgteam-g", 0)
	require.NoError(t, usergroup.AddUserToUserGroup(ctx, g.ID, user4.ID))
	require.NoError(t, organization.AddUserGroupToTeam(ctx, team.ID, g.ID, team.OrgID))

	inOrgTeam, err := organization.IsUserInAnyOrgTeamViaUserGroups(ctx, team.OrgID, user4.ID)
	require.NoError(t, err)
	assert.True(t, inOrgTeam)

	// User not in any group → should be false.
	user29 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 29})
	notIn, err := organization.IsUserInAnyOrgTeamViaUserGroups(ctx, team.OrgID, user29.ID)
	require.NoError(t, err)
	assert.False(t, notIn)
}

// TestGetUserRepoTeamsWithGroups verifies that a user in a user group (child)
// whose team assigns the parent group appears in GetUserRepoTeamsWithGroups.
func TestGetUserRepoTeamsWithGroups(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// team 2 (org 3) has repo 3 (see fixtures/team_repo.yml).
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	user8 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8})

	parent := makeGroup(t, "rteam-parent", 0)
	child := makeGroup(t, "rteam-child", parent.ID)

	require.NoError(t, usergroup.AddUserToUserGroup(ctx, child.ID, user8.ID))
	require.NoError(t, organization.AddUserGroupToTeam(ctx, team.ID, parent.ID, team.OrgID))

	// repoID 3 is in team 2 (org 3).
	teams, err := organization.GetUserRepoTeamsWithGroups(ctx, team.OrgID, user8.ID, 3)
	require.NoError(t, err)

	found := false
	for _, tm := range teams {
		if tm.ID == team.ID {
			found = true
		}
	}
	assert.True(t, found, "user in child group should see team that assigns parent group for repo")
}
