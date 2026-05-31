// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestCreateOrUpdateIssueWatch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	assert.NoError(t, issues_model.CreateOrUpdateIssueWatch(t.Context(), 3, 1, true))
	iw := unittest.AssertExistsAndLoadBean(t, &issues_model.IssueWatch{UserID: 3, IssueID: 1})
	assert.True(t, iw.IsWatching)

	assert.NoError(t, issues_model.CreateOrUpdateIssueWatch(t.Context(), 1, 1, false))
	iw = unittest.AssertExistsAndLoadBean(t, &issues_model.IssueWatch{UserID: 1, IssueID: 1})
	assert.False(t, iw.IsWatching)
}

func TestGetIssueWatch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	_, exists, err := issues_model.GetIssueWatch(t.Context(), 9, 1)
	assert.True(t, exists)
	assert.NoError(t, err)

	iw, exists, err := issues_model.GetIssueWatch(t.Context(), 2, 2)
	assert.True(t, exists)
	assert.NoError(t, err)
	assert.False(t, iw.IsWatching)

	_, exists, err = issues_model.GetIssueWatch(t.Context(), 3, 1)
	assert.False(t, exists)
	assert.NoError(t, err)
}

func TestGetIssueWatchers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	iws, err := issues_model.GetIssueWatchers(t.Context(), 1, db.ListOptions{})
	assert.NoError(t, err)
	// Watcher is inactive, thus 0
	assert.Empty(t, iws)

	iws, err = issues_model.GetIssueWatchers(t.Context(), 2, db.ListOptions{})
	assert.NoError(t, err)
	// Watcher is explicit not watching
	assert.Empty(t, iws)

	iws, err = issues_model.GetIssueWatchers(t.Context(), 5, db.ListOptions{})
	assert.NoError(t, err)
	// Issue has no Watchers
	assert.Empty(t, iws)

	iws, err = issues_model.GetIssueWatchers(t.Context(), 7, db.ListOptions{})
	assert.NoError(t, err)
	// Issue has one watcher
	assert.Len(t, iws, 1)
}

func TestGetIssueSubscribers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	checkSubscribers := func(issueID int64, expectedNames []string, expectedCount int64) {
		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issueID})
		users, err := issues_model.GetIssueSubscribers(t.Context(), issue, db.ListOptions{})
		assert.NoError(t, err)
		names := make([]string, 0, len(users))
		for _, user := range users {
			names = append(names, user.Name)
		}
		assert.ElementsMatch(t, expectedNames, names)

		count, err := issues_model.CountIssueSubscribers(t.Context(), issue)
		assert.NoError(t, err)
		assert.Equal(t, expectedCount, count)
	}

	checkSubscribers(1, []string{"user1", "user4", "user5", "user11"}, 4)
	checkSubscribers(7, []string{"user2"}, 1)
}
