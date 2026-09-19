// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewRequestRetractOwnApproval(t *testing.T) {
	for _, tc := range []struct {
		name       string
		reviewType issues_model.ReviewType
		archived   bool
		allowed    bool
	}{
		{"approval", issues_model.ReviewTypeApprove, false, true},
		{"comment", issues_model.ReviewTypeComment, false, false},
		{"archived repository", issues_model.ReviewTypeApprove, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, unittest.PrepareTestDatabase())

			pull := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{IssueID: 2})
			pull.HasMerged = false
			require.NoError(t, pull.UpdateCols(t.Context(), "has_merged"))
			issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
			require.NoError(t, issue.LoadRepo(t.Context()))
			require.NoError(t, issue.Repo.LoadOwner(t.Context()))
			reviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

			review, _, err := issues_model.SubmitReview(t.Context(), reviewer, issue, tc.reviewType, "review", "", false, nil)
			require.NoError(t, err)
			assert.False(t, review.Official)
			issue.Repo.IsArchived = tc.archived

			_, err = ReviewRequest(t.Context(), issue, reviewer, nil, reviewer, true)
			if !tc.allowed {
				assert.True(t, issues_model.IsErrNotValidReviewRequest(err))
				return
			}
			require.NoError(t, err)

			review, err = issues_model.GetReviewByIssueIDAndUserID(t.Context(), issue.ID, reviewer.ID)
			require.NoError(t, err)
			assert.Equal(t, issues_model.ReviewTypeRequest, review.Type)
			assert.False(t, review.Official)
		})
	}
}
