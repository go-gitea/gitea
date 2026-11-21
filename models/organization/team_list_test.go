// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization_test

import (
	"testing"

	"gitea.dev/models/db"
	org_model "gitea.dev/models/organization"
	"gitea.dev/models/perm"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetTeamsByIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// 1 owner team, 2 normal team
	teams, err := org_model.GetTeamsByIDs(t.Context(), []int64{1, 2})
	assert.NoError(t, err)
	assert.Len(t, teams, 2)
	assert.Equal(t, "Owners", teams[1].Name)
	assert.Equal(t, "team1", teams[2].Name)
}

func TestFilterVisibleTeams(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// org3 teams: Owners (1, user2) and team1 (2, user4) are private, test_team (7) is limited, team12Creators (12) is public
	_, err := db.GetEngine(t.Context()).ID(7).Cols("visibility").Update(&org_model.Team{Visibility: structs.VisibleTypeLimited})
	require.NoError(t, err)
	_, err = db.GetEngine(t.Context()).ID(12).Cols("visibility").Update(&org_model.Team{Visibility: structs.VisibleTypePublic})
	require.NoError(t, err)

	visibleTeamIDs := func(t *testing.T, userID int64) (ids []int64) {
		teams, err := org_model.GetTeamsByIDs(t.Context(), []int64{1, 2, 7, 12})
		require.NoError(t, err)
		viewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: userID})
		visible, err := org_model.FilterVisibleTeams(t.Context(), 3, viewer, org_model.TeamList{teams[1], teams[2], teams[7], teams[12]})
		require.NoError(t, err)
		for _, team := range visible {
			ids = append(ids, team.ID)
		}
		return ids
	}

	assert.Equal(t, []int64{1, 2, 7, 12}, visibleTeamIDs(t, 1)) // site admin
	assert.Equal(t, []int64{1, 2, 7, 12}, visibleTeamIDs(t, 2)) // org owner
	assert.Equal(t, []int64{2, 7, 12}, visibleTeamIDs(t, 4))    // org member in team1
	assert.Equal(t, []int64{12}, visibleTeamIDs(t, 5))          // not an org member
}

func TestTeamList_UnitMaxAccess(t *testing.T) {
	ctx := t.Context()
	adminTeam := &org_model.Team{AccessMode: perm.AccessModeAdmin, Units: nil}
	writeTeam := &org_model.Team{AccessMode: perm.AccessModeWrite, Units: nil}
	granularTeam := &org_model.Team{
		AccessMode: perm.AccessModeNone,
		Units: []*org_model.TeamUnit{
			{Type: unit.TypeCode, AccessMode: perm.AccessModeWrite},
		},
	}

	assert.Equal(t, perm.AccessModeAdmin, org_model.TeamList{adminTeam}.AnyRepoUnitMaxAccess(ctx, unit.TypeActions))
	assert.Equal(t, perm.AccessModeWrite, org_model.TeamList{writeTeam}.AnyRepoUnitMaxAccess(ctx, unit.TypeActions))
	assert.Equal(t, perm.AccessModeWrite, org_model.TeamList{granularTeam}.AnyRepoUnitMaxAccess(ctx, unit.TypeCode))
	assert.Equal(t, perm.AccessModeNone, org_model.TeamList{granularTeam}.AnyRepoUnitMaxAccess(ctx, unit.TypeActions))
	assert.Equal(t, perm.AccessModeAdmin, org_model.TeamList{granularTeam, adminTeam}.AnyRepoUnitMaxAccess(ctx, unit.TypeActions))
}
