// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateIssueDependency(t *testing.T) {
	// Prepare
	assert.NoError(t, PrepareTestDatabase())

	user1, err := GetUserByID(1)
	assert.NoError(t, err)

	issue1, err := GetIssueByID(1)
	assert.NoError(t, err)

	issue2, err := GetIssueByID(2)
	assert.NoError(t, err)

	// Create a dependency and check if it was successful
	err = CreateIssueDependency(user1, issue1, issue2)
	assert.NoError(t, err)

	// Do it again to see if it will check if the dependency already exists
	err = CreateIssueDependency(user1, issue1, issue2)
	assert.Error(t, err)
	assert.True(t, IsErrDependencyExists(err))

	// Check for circular dependencies
	err = CreateIssueDependency(user1, issue2, issue1)
	assert.Error(t, err)
	assert.True(t, IsErrCircularDependency(err))

	_ = AssertExistsAndLoadBean(t, &Comment{Type: CommentTypeAddDependency, PosterID: user1.ID, IssueID: issue1.ID})

	// Check if dependencies left is correct
	left, err := IssueNoDependenciesLeft(issue1)
	assert.NoError(t, err)
	assert.False(t, left)

	// Close #2 and check again
	_, err = issue2.ChangeStatus(user1, true)
	assert.NoError(t, err)

	left, err = IssueNoDependenciesLeft(issue1)
	assert.NoError(t, err)
	assert.True(t, left)

	// Test removing the dependency
	err = RemoveIssueDependency(user1, issue1, issue2, DependencyTypeBlockedBy)
	assert.NoError(t, err)
}
