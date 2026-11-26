// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

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
		OrgID     int64 `xorm:"INDEX"`
		Authorize int
	}
	type TeamUnit struct {
		ID         int64 `xorm:"pk autoincr"`
		OrgID      int64
		TeamID     int64 `xorm:"UNIQUE(s)"`
		Type       int   `xorm:"UNIQUE(s)"`
		AccessMode int
	}

	engine, deferrable := migrationtest.PrepareTestEnv(t, 0, new(Repository), new(RepoUnit), new(Team), new(TeamUnit))
	defer deferrable()
	if engine == nil || t.Failed() {
		return
	}
	t.Cleanup(test.MockVariableValue(&setting.Repository.DefaultRepoUnits, []string{"repo.code", " REPO.Security_Advisories "}))

	_, err := engine.Insert(
		&Repository{ID: 1}, &Repository{ID: 2, IsFork: true}, &Repository{ID: 3, IsMirror: true}, &Repository{ID: 4},
		&RepoUnit{RepoID: 1, Type: 1},
		&RepoUnit{RepoID: 4, Type: 11, Config: `{"PrivateVulnerabilityReporting":true}`},
		&Team{ID: 1, OrgID: 10}, &Team{ID: 2, OrgID: 10}, &Team{ID: 3, OrgID: 10, Authorize: 4}, &Team{ID: 4, OrgID: 10}, &Team{ID: 5, OrgID: 10},
		&TeamUnit{OrgID: 10, TeamID: 1, Type: 1, AccessMode: 1},
		&TeamUnit{OrgID: 10, TeamID: 2, Type: 11, AccessMode: 4},
		&TeamUnit{OrgID: 10, TeamID: 4, Type: 1, AccessMode: 0},
		&TeamUnit{OrgID: 10, TeamID: 5, Type: 6, AccessMode: 1},
	)
	require.NoError(t, err)
	require.NoError(t, AddSecurityAdvisoryTables(t.Context(), engine))

	var units []*RepoUnit
	require.NoError(t, engine.Where("type = 11").OrderBy("repo_id").Find(&units))
	require.Len(t, units, 2)
	assert.EqualValues(t, 1, units[0].RepoID)
	assert.Equal(t, "{}", units[0].Config)
	assert.EqualValues(t, 4, units[1].RepoID)
	assert.JSONEq(t, `{"PrivateVulnerabilityReporting":true}`, units[1].Config)

	var teamUnits []*TeamUnit
	require.NoError(t, engine.Where("type = 11").OrderBy("team_id").Find(&teamUnits))
	require.Len(t, teamUnits, 2)
	assert.Equal(t, TeamUnit{ID: teamUnits[0].ID, OrgID: 10, TeamID: 1, Type: 11, AccessMode: 1}, *teamUnits[0])
	assert.Equal(t, 4, teamUnits[1].AccessMode)

	exist, err := engine.IsTableExist("security_advisory_label")
	require.NoError(t, err)
	assert.True(t, exist)

	tables, err := engine.DBMetas()
	require.NoError(t, err)
	for _, table := range tables {
		if table.Name == "team" {
			assert.NotEmpty(t, table.Indexes, "adding a column must keep the indexes of the team table")
		}
	}
}
