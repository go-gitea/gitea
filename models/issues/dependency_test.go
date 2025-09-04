// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestCreateIssueDependency(t *testing.T) {
	// Prepare
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1, err := user_model.GetUserByID(t.Context(), 1)
	assert.NoError(t, err)

	issue1, err := issues_model.GetIssueByID(t.Context(), 1)
	assert.NoError(t, err)

	issue2, err := issues_model.GetIssueByID(t.Context(), 2)
	assert.NoError(t, err)

	// Create a dependency and check if it was successful
	err = issues_model.CreateIssueDependency(t.Context(), user1, issue1, issue2)
	assert.NoError(t, err)

	// Do it again to see if it will check if the dependency already exists
	err = issues_model.CreateIssueDependency(t.Context(), user1, issue1, issue2)
	assert.Error(t, err)
	assert.True(t, issues_model.IsErrDependencyExists(err))

	// Check for circular dependencies
	err = issues_model.CreateIssueDependency(t.Context(), user1, issue2, issue1)
	assert.Error(t, err)
	assert.True(t, issues_model.IsErrCircularDependency(err))

	_ = unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{Type: issues_model.CommentTypeAddDependency, PosterID: user1.ID, IssueID: issue1.ID})

	// Check if dependencies left is correct
	left, err := issues_model.IssueNoDependenciesLeft(t.Context(), issue1)
	assert.NoError(t, err)
	assert.False(t, left)

	// Close #2 and check again
	_, err = issues_model.CloseIssue(t.Context(), issue2, user1)
	assert.NoError(t, err)

	issue2Closed, err := issues_model.GetIssueByID(t.Context(), 2)
	assert.NoError(t, err)
	assert.True(t, issue2Closed.IsClosed)

	left, err = issues_model.IssueNoDependenciesLeft(t.Context(), issue1)
	assert.NoError(t, err)
	assert.True(t, left)

	// Test removing the dependency
	err = issues_model.RemoveIssueDependency(t.Context(), user1, issue1, issue2, issues_model.DependencyTypeBlockedBy)
	assert.NoError(t, err)

	_, err = issues_model.ReopenIssue(t.Context(), issue2, user1)
	assert.NoError(t, err)

	issue2Reopened, err := issues_model.GetIssueByID(t.Context(), 2)
	assert.NoError(t, err)
	assert.False(t, issue2Reopened.IsClosed)
}
