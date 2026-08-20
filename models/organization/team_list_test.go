// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization_test

import (
	"testing"

	org_model "gitea.dev/models/organization"
	"gitea.dev/models/perm"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
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
