// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db_test

import (
	"errors"
	"testing"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanRows(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	want, err := db.GetEngine(t.Context()).Count(&repo_model.RepoUnit{})
	require.NoError(t, err)
	require.Positive(t, want, "fixtures must hold at least one row for this test to mean anything")

	t.Run("visits every row", func(t *testing.T) {
		rows, err := db.GetEngine(t.Context()).Table("repo_unit").Rows(new(repo_model.RepoUnit))
		require.NoError(t, err)

		var seen int64
		require.NoError(t, db.ScanRows(rows, func(unit *repo_model.RepoUnit) error {
			assert.NotZero(t, unit.ID)
			seen++
			return nil
		}))
		assert.Equal(t, want, seen)
	})

	t.Run("returns the callback error and stops", func(t *testing.T) {
		rows, err := db.GetEngine(t.Context()).Table("repo_unit").Rows(new(repo_model.RepoUnit))
		require.NoError(t, err)

		stop := errors.New("stop")
		var seen int64
		err = db.ScanRows(rows, func(unit *repo_model.RepoUnit) error {
			seen++
			return stop
		})
		assert.ErrorIs(t, err, stop)
		assert.Equal(t, int64(1), seen)
	})
}
