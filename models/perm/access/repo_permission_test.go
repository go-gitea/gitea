// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package access

import (
	"testing"

	perm_model "code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestHasAnyUnitAccess(t *testing.T) {
	perm := Permission{}
	assert.False(t, perm.HasAnyUnitAccess())

	perm = Permission{
		units: []*repo_model.RepoUnit{{Type: unit.TypeWiki}},
	}
	assert.False(t, perm.HasAnyUnitAccess())
	assert.False(t, perm.HasAnyUnitAccessOrEveryoneAccess())

	perm = Permission{
		units:              []*repo_model.RepoUnit{{Type: unit.TypeWiki}},
		everyoneAccessMode: map[unit.Type]perm_model.AccessMode{unit.TypeIssues: perm_model.AccessModeRead},
	}
	assert.False(t, perm.HasAnyUnitAccess())
	assert.True(t, perm.HasAnyUnitAccessOrEveryoneAccess())

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

func TestApplyEveryoneRepoPermission(t *testing.T) {
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
