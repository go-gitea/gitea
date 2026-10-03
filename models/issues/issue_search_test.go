// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/optional"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssues_FilterWIP(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// fixture issue 2 is a pull in repo 1, mark it as WIP with a lowercase prefix on purpose
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	issue.Title = "[wip] issue2"
	_, err := db.GetEngine(t.Context()).ID(issue.ID).Cols("name").Update(issue)
	require.NoError(t, err)

	base := &issues_model.IssuesOptions{
		RepoIDs: []int64{1},
		IsPull:  optional.Some(true),
	}

	only, err := issues_model.Issues(t.Context(), base.Copy(func(o *issues_model.IssuesOptions) {
		o.IsWIP = optional.Some(true)
	}))
	require.NoError(t, err)
	assert.NotEmpty(t, only)
	for _, issue := range only {
		assert.True(t, issues_model.HasWorkInProgressPrefix(issue.Title), issue.Title)
	}

	hidden, err := issues_model.Issues(t.Context(), base.Copy(func(o *issues_model.IssuesOptions) {
		o.IsWIP = optional.Some(false)
	}))
	require.NoError(t, err)
	for _, issue := range hidden {
		assert.False(t, issues_model.HasWorkInProgressPrefix(issue.Title), issue.Title)
	}

	all, err := issues_model.Issues(t.Context(), base)
	require.NoError(t, err)
	assert.Len(t, all, len(only)+len(hidden))
}
