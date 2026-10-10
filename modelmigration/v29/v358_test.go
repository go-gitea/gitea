// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddSecurityAdvisoryTables(t *testing.T) {
	type Repository struct {
		ID       int64 `xorm:"pk"`
		IsFork   bool
		IsMirror bool
	}
	type RepoUnit struct {
		ID                  int64 `xorm:"pk autoincr"`
		RepoID              int64
		Type                int
		Config              string `xorm:"TEXT"`
		CreatedUnix         int64
		AnonymousAccessMode int `xorm:"NOT NULL DEFAULT 0"`
		EveryoneAccessMode  int `xorm:"NOT NULL DEFAULT 0"`
	}
	type Team struct {
		ID        int64 `xorm:"pk"`
		OrgID     int64
		Authorize int
	}
	type TeamUnit struct {
		ID         int64 `xorm:"pk autoincr"`
		OrgID      int64
		TeamID     int64 `xorm:"UNIQUE(s)"`
		Type       int   `xorm:"UNIQUE(s)"`
		AccessMode int
	}

	x, deferrable := migrationtest.PrepareTestEnv(t, 0, new(Repository), new(RepoUnit), new(Team), new(TeamUnit))
	defer deferrable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(
		&Repository{ID: 1},
		&Repository{ID: 2, IsFork: true},
		&Repository{ID: 3, IsMirror: true},
		&Repository{ID: 4},
		&RepoUnit{RepoID: 1, Type: 1},
		&RepoUnit{RepoID: 4, Type: 11, Config: `{"PrivateVulnerabilityReporting":true}`},
		&Team{ID: 1, OrgID: 10},
		&Team{ID: 2, OrgID: 10},
		&Team{ID: 3, OrgID: 10, Authorize: 4},
		&Team{ID: 4, OrgID: 10},
		&Team{ID: 5, OrgID: 10},
		&TeamUnit{OrgID: 10, TeamID: 1, Type: 1, AccessMode: 1},
		&TeamUnit{OrgID: 10, TeamID: 2, Type: 11, AccessMode: 4},
		&TeamUnit{OrgID: 10, TeamID: 4, Type: 1, AccessMode: 0},
		&TeamUnit{OrgID: 10, TeamID: 5, Type: 6, AccessMode: 1},
	)
	require.NoError(t, err)
	require.NoError(t, AddSecurityAdvisoryTables(t.Context(), x))

	var units []*RepoUnit
	require.NoError(t, x.Where("type = 11").OrderBy("repo_id").Find(&units))
	require.Len(t, units, 2)
	assert.EqualValues(t, 1, units[0].RepoID)
	assert.Equal(t, "{}", units[0].Config)
	assert.EqualValues(t, 4, units[1].RepoID)
	assert.JSONEq(t, `{"PrivateVulnerabilityReporting":true}`, units[1].Config)

	// only team 1 is new: team 2 already had the unit, team 3 has a general permission, teams 4 and 5 can't read the code
	var teamUnits []*TeamUnit
	require.NoError(t, x.Where("type = 11").OrderBy("team_id").Find(&teamUnits))
	require.Len(t, teamUnits, 2)
	assert.Equal(t, TeamUnit{ID: teamUnits[0].ID, OrgID: 10, TeamID: 1, Type: 11, AccessMode: 1}, *teamUnits[0])
	assert.Equal(t, 4, teamUnits[1].AccessMode)

	exist, err := x.IsTableExist("security_advisory_label")
	require.NoError(t, err)
	assert.True(t, exist)
}
