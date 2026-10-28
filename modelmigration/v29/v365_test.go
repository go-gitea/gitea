// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// actionRunJobBeforeV365 mirrors the columns of action_run_job that matter here, with the
// index on is_reusable_caller that the migration drops.
type actionRunJobBeforeV365 struct {
	ID               int64 `xorm:"pk autoincr"`
	RepoID           int64 `xorm:"index"`
	Status           int   `xorm:"index"`
	IsReusableCaller bool  `xorm:"index NOT NULL DEFAULT FALSE"`
}

func (actionRunJobBeforeV365) TableName() string { return "action_run_job" }

func TestDropActionRunJobReusableCallerIndex(t *testing.T) {
	x, deferrable := migrationtest.PrepareTestEnv(t, 0, new(actionRunJobBeforeV365))
	defer deferrable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(
		&actionRunJobBeforeV365{RepoID: 1, Status: 5, IsReusableCaller: false},
		&actionRunJobBeforeV365{RepoID: 1, Status: 7, IsReusableCaller: true},
	)
	require.NoError(t, err)

	indexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "action_run_job")
	require.NoError(t, err)
	require.Contains(t, indexes, "is_reusable_caller")

	require.NoError(t, DropActionRunJobReusableCallerIndex(t.Context(), x))

	indexes, err = x.Dialect().GetIndexes(x.DB(), context.Background(), "action_run_job")
	require.NoError(t, err)
	assert.NotContains(t, indexes, "is_reusable_caller")
	assert.Contains(t, indexes, "repo_id", "other indexes stay")
	assert.Contains(t, indexes, "status", "other indexes stay")

	count, err := x.Table("action_run_job").Count()
	require.NoError(t, err)
	assert.EqualValues(t, 2, count, "rows stay")

	// running it again, with the index already gone, is a no-op
	require.NoError(t, DropActionRunJobReusableCallerIndex(t.Context(), x))
}
