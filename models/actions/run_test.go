// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestUpdateRepoRunsNumbers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// update the number to a wrong one, the original is 3
	_, err := db.GetEngine(t.Context()).ID(4).Cols("num_closed_action_runs").Update(&repo_model.Repository{
		NumClosedActionRuns: 2,
	})
	assert.NoError(t, err)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	assert.Equal(t, 4, repo.NumActionRuns)
	assert.Equal(t, 2, repo.NumClosedActionRuns)

	// now update will correct them, only num_actionr_runs and num_closed_action_runs should be updated
	err = UpdateRepoRunsNumbers(t.Context(), repo)
	assert.NoError(t, err)
	repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	assert.Equal(t, 4, repo.NumActionRuns)
	assert.Equal(t, 3, repo.NumClosedActionRuns)
}

func TestActionRun_Duration_NonNegative(t *testing.T) {
	run := &ActionRun{
		Started:          timeutil.TimeStamp(100),
		Stopped:          timeutil.TimeStamp(200),
		Status:           StatusSuccess,
		PreviousDuration: -time.Hour,
	}
	assert.Equal(t, time.Duration(0), run.Duration())
}
