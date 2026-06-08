// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package usergroup_test

import (
	"testing"

	"gitea.dev/models/organization"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/models/usergroup"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserGroupMemberCounts(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	g := makeGroup(t, "count-g", 0)
	require.NoError(t, usergroup.AddUserToUserGroup(ctx, g.ID, user4.ID))
	require.NoError(t, usergroup.AddUserToUserGroup(ctx, g.ID, user5.ID))

	counts, err := usergroup.GetUserGroupMemberCounts(ctx, []int64{g.ID})
	require.NoError(t, err)
	assert.Equal(t, int64(2), counts[g.ID])

	// Empty input should return empty map without error.
	empty, err := usergroup.GetUserGroupMemberCounts(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

// TestUserGroupTeamMembership verifies that a user in a child group is
// considered a member of a team that assigns the parent group.
func TestUserGroupTeamMembership(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})

	parent := makeGroup(t, "tm-parent", 0)
	child := makeGroup(t, "tm-child", parent.ID)

	require.NoError(t, usergroup.AddUserToUserGroup(ctx, child.ID, user.ID))
	require.NoError(t, organization.AddUserGroupToTeam(ctx, team.ID, parent.ID, team.OrgID))

	// User in child group → team assigns parent → should be a member.
	isMember, err := organization.IsTeamMemberWithGroups(ctx, team.OrgID, team.ID, user.ID)
	require.NoError(t, err)
	assert.True(t, isMember)

	// Not a member of a completely unrelated team.
	isMemberOther, err := organization.IsTeamMemberWithGroups(ctx, team.OrgID, team.ID+100, user.ID)
	require.NoError(t, err)
	assert.False(t, isMemberOther)

	// GetTeamMembersWithGroups must include the user.
	members, err := organization.GetTeamMembersWithGroups(ctx, &organization.SearchMembersOptions{TeamID: team.ID})
	require.NoError(t, err)
	assert.True(t, containsUser(members, user.ID))

	// GetUserOrgTeams must return the team.
	teams, err := organization.GetUserOrgTeams(ctx, team.OrgID, user.ID)
	require.NoError(t, err)
	found := false
	for _, tm := range teams {
		if tm.ID == team.ID {
			found = true
		}
	}
	assert.True(t, found, "GetUserOrgTeams should include the team assigned via parent group")
}
