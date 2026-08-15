// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

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

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(Team))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(
		&Team{ID: 1, Authorize: 4},
		&Team{ID: 2, Authorize: 3},
		&Team{ID: 3, Authorize: 2},
		&Team{ID: 4, Authorize: 1},
		&Team{ID: 5, Authorize: 0},
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
	assert.Equal(t, 0, get(3))
	assert.Equal(t, 0, get(4))
	assert.Equal(t, 0, get(5))
}
