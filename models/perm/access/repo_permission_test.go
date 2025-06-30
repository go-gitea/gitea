// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package access

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	perm_model "code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasAnyUnitAccess(t *testing.T) {
	perm := Permission{}
	assert.False(t, perm.HasAnyUnitAccess())

	perm = Permission{
		units: []*repo_model.RepoUnit{{Type: unit.TypeWiki}},
	}
	assert.False(t, perm.HasAnyUnitAccess())
	assert.False(t, perm.HasAnyUnitAccessOrPublicAccess())

	perm = Permission{
		units:              []*repo_model.RepoUnit{{Type: unit.TypeWiki}},
		everyoneAccessMode: map[unit.Type]perm_model.AccessMode{unit.TypeIssues: perm_model.AccessModeRead},
	}
	assert.False(t, perm.HasAnyUnitAccess())
	assert.True(t, perm.HasAnyUnitAccessOrPublicAccess())

	perm = Permission{
		units:               []*repo_model.RepoUnit{{Type: unit.TypeWiki}},
		anonymousAccessMode: map[unit.Type]perm_model.AccessMode{unit.TypeIssues: perm_model.AccessModeRead},
	}
	assert.False(t, perm.HasAnyUnitAccess())
	assert.True(t, perm.HasAnyUnitAccessOrPublicAccess())

	perm = Permission{
		AccessMode: perm_model.AccessModeRead,
		units:      []*repo_model.RepoUnit{{Type: unit.TypeWiki}},
	}
	assert.True(t, perm.HasAnyUnitAccess())

	perm = Permission{
		unitsMode: map[unit.Type]perm_model.AccessMode{unit.TypeWiki: perm_model.AccessModeRead},
	}
	assert.True(t, perm.HasAnyUnitAccess())
}

func TestApplyPublicAccessRepoPermission(t *testing.T) {
	perm := Permission{
		AccessMode: perm_model.AccessModeNone,
		units: []*repo_model.RepoUnit{
			{Type: unit.TypeWiki, EveryoneAccessMode: perm_model.AccessModeRead},
		},
	}
	finalProcessRepoUnitPermission(nil, &perm)
	assert.False(t, perm.CanRead(unit.TypeWiki))

	perm = Permission{
		AccessMode: perm_model.AccessModeNone,
		units: []*repo_model.RepoUnit{
			{Type: unit.TypeWiki, AnonymousAccessMode: perm_model.AccessModeRead},
		},
	}
	finalProcessRepoUnitPermission(nil, &perm)
	assert.True(t, perm.CanRead(unit.TypeWiki))

	perm = Permission{
		AccessMode: perm_model.AccessModeNone,
		units: []*repo_model.RepoUnit{
			{Type: unit.TypeWiki, EveryoneAccessMode: perm_model.AccessModeRead},
		},
	}
	finalProcessRepoUnitPermission(&user_model.User{ID: 0}, &perm)
	assert.False(t, perm.CanRead(unit.TypeWiki))

	perm = Permission{
		AccessMode: perm_model.AccessModeNone,
		units: []*repo_model.RepoUnit{
			{Type: unit.TypeWiki, EveryoneAccessMode: perm_model.AccessModeRead},
		},
	}
	finalProcessRepoUnitPermission(&user_model.User{ID: 1}, &perm)
	assert.True(t, perm.CanRead(unit.TypeWiki))

	perm = Permission{
		AccessMode: perm_model.AccessModeWrite,
		units: []*repo_model.RepoUnit{
			{Type: unit.TypeWiki, EveryoneAccessMode: perm_model.AccessModeRead},
		},
	}
	finalProcessRepoUnitPermission(&user_model.User{ID: 1}, &perm)
	// it should work the same as "EveryoneAccessMode: none" because the default AccessMode should be applied to units
	assert.True(t, perm.CanWrite(unit.TypeWiki))

	perm = Permission{
		units: []*repo_model.RepoUnit{
			{Type: unit.TypeCode}, // will be removed
			{Type: unit.TypeWiki, EveryoneAccessMode: perm_model.AccessModeRead},
		},
		unitsMode: map[unit.Type]perm_model.AccessMode{
			unit.TypeWiki: perm_model.AccessModeWrite,
		},
	}
	finalProcessRepoUnitPermission(&user_model.User{ID: 1}, &perm)
	assert.True(t, perm.CanWrite(unit.TypeWiki))
	assert.Len(t, perm.units, 1)
}

func TestUnitAccessMode(t *testing.T) {
	perm := Permission{
		AccessMode: perm_model.AccessModeNone,
	}
	assert.Equal(t, perm_model.AccessModeNone, perm.UnitAccessMode(unit.TypeWiki), "no unit, no map, use AccessMode")

	perm = Permission{
		AccessMode: perm_model.AccessModeRead,
		units: []*repo_model.RepoUnit{
			{Type: unit.TypeWiki},
		},
	}
	assert.Equal(t, perm_model.AccessModeRead, perm.UnitAccessMode(unit.TypeWiki), "only unit, no map, use AccessMode")

	perm = Permission{
		AccessMode: perm_model.AccessModeAdmin,
		unitsMode: map[unit.Type]perm_model.AccessMode{
			unit.TypeWiki: perm_model.AccessModeRead,
		},
	}
	assert.Equal(t, perm_model.AccessModeAdmin, perm.UnitAccessMode(unit.TypeWiki), "no unit, only map, admin overrides map")

	perm = Permission{
		AccessMode: perm_model.AccessModeNone,
		unitsMode: map[unit.Type]perm_model.AccessMode{
			unit.TypeWiki: perm_model.AccessModeRead,
		},
	}
	assert.Equal(t, perm_model.AccessModeRead, perm.UnitAccessMode(unit.TypeWiki), "no unit, only map, use map")

	perm = Permission{
		AccessMode: perm_model.AccessModeNone,
		units: []*repo_model.RepoUnit{
			{Type: unit.TypeWiki},
		},
		unitsMode: map[unit.Type]perm_model.AccessMode{
			unit.TypeWiki: perm_model.AccessModeRead,
		},
	}
	assert.Equal(t, perm_model.AccessModeRead, perm.UnitAccessMode(unit.TypeWiki), "has unit, and map, use map")
}

func TestGetUserRepoPermission(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	repo32 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 32}) // org public repo
	require.NoError(t, repo32.LoadOwner(ctx))
	require.True(t, repo32.Owner.IsOrganization())

	require.NoError(t, db.TruncateBeans(ctx, &organization.Team{}, &organization.TeamUser{}, &organization.TeamRepo{}, &organization.TeamUnit{}))
	org := repo32.Owner
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	team := &organization.Team{OrgID: org.ID, LowerName: "test_team"}
	require.NoError(t, db.Insert(ctx, team))

	t.Run("DoerInTeamWithNoRepo", func(t *testing.T) {
		require.NoError(t, db.Insert(ctx, &organization.TeamUser{OrgID: org.ID, TeamID: team.ID, UID: user.ID}))
		perm, err := GetUserRepoPermission(ctx, repo32, user)
		require.NoError(t, err)
		assert.Equal(t, perm_model.AccessModeRead, perm.AccessMode)
		assert.Nil(t, perm.unitsMode) // doer in the team, but has no access to the repo
	})

	require.NoError(t, db.Insert(ctx, &organization.TeamRepo{OrgID: org.ID, TeamID: team.ID, RepoID: repo32.ID}))
	require.NoError(t, db.Insert(ctx, &organization.TeamUnit{OrgID: org.ID, TeamID: team.ID, Type: unit.TypeCode, AccessMode: perm_model.AccessModeNone}))
	t.Run("DoerWithTeamUnitAccessNone", func(t *testing.T) {
		perm, err := GetUserRepoPermission(ctx, repo32, user)
		require.NoError(t, err)
		assert.Equal(t, perm_model.AccessModeRead, perm.AccessMode)
		assert.Equal(t, perm_model.AccessModeRead, perm.unitsMode[unit.TypeCode])
		assert.Equal(t, perm_model.AccessModeRead, perm.unitsMode[unit.TypeIssues])
	})

	require.NoError(t, db.TruncateBeans(ctx, &organization.TeamUnit{}))
	require.NoError(t, db.Insert(ctx, &organization.TeamUnit{OrgID: org.ID, TeamID: team.ID, Type: unit.TypeCode, AccessMode: perm_model.AccessModeWrite}))
	t.Run("DoerWithTeamUnitAccessWrite", func(t *testing.T) {
		perm, err := GetUserRepoPermission(ctx, repo32, user)
		require.NoError(t, err)
		assert.Equal(t, perm_model.AccessModeRead, perm.AccessMode)
		assert.Equal(t, perm_model.AccessModeWrite, perm.unitsMode[unit.TypeCode])
		assert.Equal(t, perm_model.AccessModeRead, perm.unitsMode[unit.TypeIssues])
	})
}
