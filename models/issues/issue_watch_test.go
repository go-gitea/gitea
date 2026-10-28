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

func TestIssueWatch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	t.Run("GetIssueWatch", func(t *testing.T) {
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
	})

	t.Run("GetIssueWatchers", func(t *testing.T) {
		for _, issueID := range []int64{
			1, // Watcher is inactive, thus 0
			2, // Watcher is explicit not watching
			5, // Issue has no Watchers
		} {
			iws, err := issues_model.GetIssueWatchers(t.Context(), issueID, db.ListOptions{})
			assert.NoError(t, err)
			assert.Empty(t, iws)
		}

		iws, err := issues_model.GetIssueWatchers(t.Context(), 7, db.ListOptions{})
		assert.NoError(t, err)
		// Issue has one watcher
		assert.Len(t, iws, 1)
	})

	t.Run("CreateOrUpdateIssueWatch", func(t *testing.T) {
		assert.NoError(t, issues_model.CreateOrUpdateIssueWatch(t.Context(), 3, 1, true))
		iw := unittest.AssertExistsAndLoadBean(t, &issues_model.IssueWatch{UserID: 3, IssueID: 1})
		assert.True(t, iw.IsWatching)

		assert.NoError(t, issues_model.CreateOrUpdateIssueWatch(t.Context(), 1, 1, false))
		iw = unittest.AssertExistsAndLoadBean(t, &issues_model.IssueWatch{UserID: 1, IssueID: 1})
		assert.False(t, iw.IsWatching)
	})
}
