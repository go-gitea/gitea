// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization_test

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	"gitea.dev/models/perm"
	"gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTeamsWithAccessToRepoUnit(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	org41 := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 41})
	repo61 := unittest.AssertExistsAndLoadBean(t, &repo.Repository{ID: 61})

	teams, err := organization.GetTeamsWithAccessToAnyRepoUnit(t.Context(), org41.ID, repo61.ID, perm.AccessModeRead, unit.TypePullRequests)
	assert.NoError(t, err)
	if assert.Len(t, teams, 2) {
		assert.EqualValues(t, 21, teams[0].ID)
		assert.EqualValues(t, 22, teams[1].ID)
	}
}

func TestSecurityTeamMembers(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	// team 2 of org 3 has repository 3 and the members user2 and user4
	isMember, err := organization.IsSecurityTeamMember(ctx, 4, 3)
	require.NoError(t, err)
	assert.False(t, isMember)

	_, err = db.GetEngine(ctx).ID(2).Cols("is_security_team").Update(&organization.Team{IsSecurityTeam: true})
	require.NoError(t, err)
	isMember, err = organization.IsSecurityTeamMember(ctx, 4, 3)
	require.NoError(t, err)
	assert.True(t, isMember)
	isMember, err = organization.IsSecurityTeamMember(ctx, 4, 5)
	require.NoError(t, err)
	assert.False(t, isMember, "the team doesn't include repository 5")
	isMember, err = organization.IsSecurityTeamMember(ctx, 5, 3)
	require.NoError(t, err)
	assert.False(t, isMember)

	ids, err := organization.GetSecurityTeamMemberIDs(ctx, 3)
	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{2, 4}, ids)
}
