// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull_test

import (
	"testing"

	"gitea.dev/models/pull"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetScheduledAutoMergeError(t *testing.T) {
	unittest.PrepareTestEnv(t)

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	require.NoError(t, pull.ScheduleAutoMerge(t.Context(), doer, 2, repo_model.MergeStyleMerge, "merge message", false))

	require.NoError(t, pull.SetScheduledAutoMergeError(t.Context(), 2, "merge failed"))

	exist, scheduled, err := pull.GetScheduledMergeByPullID(t.Context(), 2)
	require.NoError(t, err)
	require.True(t, exist)
	assert.Equal(t, "merge failed", scheduled.ErrorMessage)
}
