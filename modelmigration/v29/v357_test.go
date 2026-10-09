// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeLegacyTeamAuthorize(t *testing.T) {
	type Team struct {
		ID        int64 `xorm:"pk"`
		Authorize int
	}
	type TeamUnit struct {
		ID     int64 `xorm:"pk"`
		TeamID int64 `xorm:"INDEX"`
	}

	x, deferrable := migrationtest.PrepareTestEnv(t, 0, new(Team), new(TeamUnit))
	defer deferrable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(
		&Team{ID: 1, Authorize: 4},
		&Team{ID: 2, Authorize: 3},
		&Team{ID: 3, Authorize: 2},
		&Team{ID: 4, Authorize: 1},
		&Team{ID: 5, Authorize: 0},

		&TeamUnit{TeamID: 3},
	)
	require.NoError(t, err)
	require.NoError(t, NormalizeLegacyTeamAuthorize(t.Context(), x))

	get := func(id int64) int {
		tBean := &Team{ID: id}
		has, err := x.Get(tBean)
		require.NoError(t, err)
		require.True(t, has)
		return tBean.Authorize
	}
	assert.Equal(t, 4, get(1))
	assert.Equal(t, 3, get(2))
	assert.Equal(t, 0, get(3)) // has team unit, reset to none
	assert.Equal(t, 1, get(4)) // no team unit, kept
	assert.Equal(t, 0, get(5))
}
