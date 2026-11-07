// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"strings"
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/require"
)

func TestAddUniqueTeamNameIndex(t *testing.T) {
	type Team struct {
		ID              int64 `xorm:"pk autoincr"`
		OrgID           int64 `xorm:"INDEX"`
		LowerName, Name string
	}

	x, cleanup := migrationtest.PrepareTestEnv(t, 0, new(Team))
	defer cleanup()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(
		&Team{OrgID: 1, Name: "Team", LowerName: "team"},
		&Team{OrgID: 1, Name: "Team", LowerName: "team"},
		&Team{OrgID: 1, Name: "Team-2", LowerName: "team-2"},
		&Team{OrgID: 2, Name: "Team", LowerName: "team"},
		&Team{OrgID: 1, Name: strings.Repeat("A", 255), LowerName: strings.Repeat("a", 255)},
		&Team{OrgID: 1, Name: strings.Repeat("A", 255), LowerName: strings.Repeat("a", 255)},
	)
	require.NoError(t, err)
	for range 2 {
		require.NoError(t, AddUniqueTeamNameIndex(t.Context(), x))
		var teams []Team
		require.NoError(t, x.Asc("id").Find(&teams))
		require.Equal(t, []Team{
			{ID: 1, OrgID: 1, Name: "Team", LowerName: "team"},
			{ID: 2, OrgID: 1, Name: "Team-3", LowerName: "team-3"},
			{ID: 3, OrgID: 1, Name: "Team-2", LowerName: "team-2"},
			{ID: 4, OrgID: 2, Name: "Team", LowerName: "team"},
			{ID: 5, OrgID: 1, Name: strings.Repeat("A", 255), LowerName: strings.Repeat("a", 255)},
			{ID: 6, OrgID: 1, Name: strings.Repeat("A", 253) + "-6", LowerName: strings.Repeat("a", 253) + "-6"},
		}, teams)
	}
	_, err = x.Insert(&Team{OrgID: 1, Name: "TEAM", LowerName: "team"})
	require.Error(t, err)
}
