// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	"github.com/stretchr/testify/assert"
)

func TestGetIssueWatchers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issueList := issues_model.IssueList{
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1}), // repo 1
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2}), // repo 1
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 5}), // repo 1
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 7}), // repo 2
	}

	iws, err := issues_model.GetIssueSubscribers(db.DefaultContext, issueList[0], db.ListOptions{})
	assert.NoError(t, err)
	// +isIssueWatching[]
	// -noIssueWatching[]
	// +participates[2,3,5]
	// +poster[1]
	// +repoWatch[1,4,9,11]
	// -inactive[3,9]
	// => [1,4,5,11]
	assert.Len(t, iws, 4)

	iws, err = issues_model.GetIssueSubscribers(db.DefaultContext, issueList[1], db.ListOptions{})
	assert.NoError(t, err)
	// +isIssueWatching[]
	// -noIssueWatching[2]
	// +participates[1]
	// +poster[1]
	// +repoWatch[1,4,9,11]
	// -inactive[3,9]
	// => [1,4,11]
	assert.Len(t, iws, 3)

	iws, err = issues_model.GetIssueSubscribers(db.DefaultContext, issueList[2], db.ListOptions{})
	assert.NoError(t, err)
	// +isIssueWatching[]
	// -noIssueWatching[]
	// +participates[]
	// +poster[2]
	// +repoWatch[1,4,9,11]
	// -inactive[3,9]
	// => [1,2,4,11]
	assert.Len(t, iws, 4)

	iws, err = issues_model.GetIssueSubscribers(db.DefaultContext, issueList[3], db.ListOptions{})
	assert.NoError(t, err)
	// +isIssueWatching[]
	// -noIssueWatching[]
	// +participates[]
	// +poster[2]
	// +repoWatch[]
	// -inactive[3,9]
	// => [2]
	assert.Len(t, iws, 1)
}
