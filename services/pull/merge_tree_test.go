// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func Test_testPullRequestMergeTree(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pull := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pull.LoadIssue(t.Context()))
	assert.NoError(t, pull.LoadBaseRepo(t.Context()))
	assert.NoError(t, pull.LoadHeadRepo(t.Context()))

	// pull 2 is mergeable, set to conflicted to see if the function updates it correctly
	pull.Status = issues_model.PullRequestStatusConflict
	pull.ConflictedFiles = []string{"old_file.go"}
	pull.ChangedProtectedFiles = []string{"protected_file.go"}

	err := testPullRequestMergeTree(t.Context(), pull)
	assert.NoError(t, err)
	assert.Equal(t, issues_model.PullRequestStatusMergeable, pull.Status)
	assert.Empty(t, pull.ConflictedFiles)
	assert.Empty(t, pull.ChangedProtectedFiles)
}
