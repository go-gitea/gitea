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
	assertMember := func(expected bool, userID, repoID int64) {
		t.Helper()
		isMember, err := organization.IsSecurityTeamMember(ctx, userID, repoID)
		require.NoError(t, err)
		assert.Equal(t, expected, isMember)
	}
	// team 2 of org 3 has repository 3 and the members user2 and user4
	assertMember(false, 4, 3)
	_, err := db.GetEngine(ctx).ID(2).Cols("is_security_team").Update(&organization.Team{IsSecurityTeam: true})
	require.NoError(t, err)
	assertMember(true, 4, 3)

	// a team with unit permissions also needs the advisories unit
	_, err = db.GetEngine(ctx).ID(2).Cols("authorize").Update(&organization.Team{AccessMode: perm.AccessModeNone})
	require.NoError(t, err)
	assertMember(false, 4, 3)
	require.NoError(t, db.Insert(ctx, &organization.TeamUnit{OrgID: 3, TeamID: 2, Type: unit.TypeSecurityAdvisories, AccessMode: perm.AccessModeRead}))
	assertMember(true, 4, 3)

	assertMember(false, 4, 5) // the team doesn't include repository 5
	assertMember(false, 5, 3)

	ids, err := organization.GetSecurityTeamMemberIDs(ctx, 3)
	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{2, 4}, ids)
}
