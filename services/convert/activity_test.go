// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"testing"

	activities_model "gitea.dev/models/activities"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
)

func TestToActivitiesLeavesOutADeletedRepository(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	// What GetFeeds hands over for an action whose repository was deleted:
	// the row, with no Repo loaded for it.
	actions := activities_model.ActionList{
		{ID: 1, RepoID: repo.ID, Repo: repo, ActUser: user, OpType: activities_model.ActionCommitRepo},
		{ID: 2, RepoID: 999999, ActUser: user, OpType: activities_model.ActionCommitRepo},
	}

	converted := ToActivities(t.Context(), actions, user)
	assert.Len(t, converted, 1)
	assert.EqualValues(t, 1, converted[0].ID)
}
