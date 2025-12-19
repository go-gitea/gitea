// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull_test

import (
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	pull_service "code.gitea.io/gitea/services/pull"

	"github.com/stretchr/testify/assert"
)

func TestDismissReview(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pull := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{})
	assert.NoError(t, pull.LoadIssue(t.Context()))
	issue := pull.Issue
	assert.NoError(t, issue.LoadRepo(t.Context()))
	reviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	review, err := issues_model.CreateReview(t.Context(), issues_model.CreateReviewOptions{
		Issue:    issue,
		Reviewer: reviewer,
		Type:     issues_model.ReviewTypeReject,
	})

	assert.NoError(t, err)
	issue.IsClosed = true
	pull.HasMerged = false
	assert.NoError(t, issues_model.UpdateIssueCols(t.Context(), issue, "is_closed"))
	assert.NoError(t, pull.UpdateCols(t.Context(), "has_merged"))
	_, err = pull_service.DismissReview(t.Context(), review.ID, issue.RepoID, "", &user_model.User{}, false, false)
	assert.Error(t, err)
	assert.True(t, pull_service.IsErrDismissRequestOnClosedPR(err))

	pull.HasMerged = true
	pull.Issue.IsClosed = false
	assert.NoError(t, issues_model.UpdateIssueCols(t.Context(), issue, "is_closed"))
	assert.NoError(t, pull.UpdateCols(t.Context(), "has_merged"))
	_, err = pull_service.DismissReview(t.Context(), review.ID, issue.RepoID, "", &user_model.User{}, false, false)
	assert.Error(t, err)
	assert.True(t, pull_service.IsErrDismissRequestOnClosedPR(err))
}
