// Copyright 2021 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeleteOrphanedObjects(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	countBefore, err := x.Count(&PullRequest{})
	assert.NoError(t, err)

	_, err = x.Insert(&PullRequest{IssueID: 1000}, &PullRequest{IssueID: 1001}, &PullRequest{IssueID: 1003})
	assert.NoError(t, err)

	orphaned, err := CountOrphanedObjects("pull_request", "issue", "pull_request.issue_id=issue.id")
	assert.NoError(t, err)
	assert.EqualValues(t, 3, orphaned)

	err = DeleteOrphanedObjects("pull_request", "issue", "pull_request.issue_id=issue.id")
	assert.NoError(t, err)

	countAfter, err := x.Count(&PullRequest{})
	assert.NoError(t, err)
	assert.EqualValues(t, countBefore, countAfter)
}
