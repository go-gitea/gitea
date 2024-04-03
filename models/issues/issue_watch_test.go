// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestCreateOrUpdateIssueWatch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	assert.NoError(t, issues_model.CreateOrUpdateIssueWatch(db.DefaultContext, 3, 1, true))
	iw := unittest.AssertExistsAndLoadBean(t, &issues_model.IssueWatch{UserID: 3, IssueID: 1})
	assert.True(t, iw.IsWatching)

	assert.NoError(t, issues_model.CreateOrUpdateIssueWatch(db.DefaultContext, 1, 1, false))
	iw = unittest.AssertExistsAndLoadBean(t, &issues_model.IssueWatch{UserID: 1, IssueID: 1})
	assert.False(t, iw.IsWatching)
}

func TestGetIssueWatch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	_, exists, err := issues_model.GetIssueWatch(db.DefaultContext, 9, 1)
	assert.True(t, exists)
	assert.NoError(t, err)

	iw, exists, err := issues_model.GetIssueWatch(db.DefaultContext, 2, 2)
	assert.True(t, exists)
	assert.NoError(t, err)
	assert.False(t, iw.IsWatching)

	_, exists, err = issues_model.GetIssueWatch(db.DefaultContext, 3, 1)
	assert.False(t, exists)
	assert.NoError(t, err)
}

func TestGetIssueWatchers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	iws, err := issues_model.GetIssueWatchers(db.DefaultContext, 1, db.ListOptions{})
	assert.NoError(t, err)
	// Watcher is inactive, thus 0
	assert.Len(t, iws, 0)

	iws, err = issues_model.GetIssueWatchers(db.DefaultContext, 2, db.ListOptions{})
	assert.NoError(t, err)
	// Watcher is explicit not watching
	assert.Len(t, iws, 0)

	iws, err = issues_model.GetIssueWatchers(db.DefaultContext, 5, db.ListOptions{})
	assert.NoError(t, err)
	// Issue has no Watchers
	assert.Len(t, iws, 0)

	iws, err = issues_model.GetIssueWatchers(db.DefaultContext, 7, db.ListOptions{})
	assert.NoError(t, err)
	// Issue has one watcher
	assert.Len(t, iws, 1)
}
