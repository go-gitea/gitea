// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestGetReviewByID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	review, err := issues_model.GetReviewByID(t.Context(), 1)
	assert.NoError(t, err)
	assert.Equal(t, "Demo Review", review.Content)
	assert.Equal(t, issues_model.ReviewTypeApprove, review.Type)

	_, err = issues_model.GetReviewByID(t.Context(), 23892)
	assert.Error(t, err)
	assert.True(t, issues_model.IsErrReviewNotExist(err), "IsErrReviewNotExist")
}

func TestReview_LoadAttributes(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	review := unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 1})
	assert.NoError(t, review.LoadAttributes(t.Context()))
	assert.NotNil(t, review.Issue)
	assert.NotNil(t, review.Reviewer)

	invalidReview1 := unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 2})
	assert.Error(t, invalidReview1.LoadAttributes(t.Context()))

	invalidReview2 := unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 3})
	assert.Error(t, invalidReview2.LoadAttributes(t.Context()))
}

func TestReview_LoadCodeComments(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	review := unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 4})
	assert.NoError(t, review.LoadAttributes(t.Context()))
	assert.NoError(t, review.LoadCodeComments(t.Context()))
	assert.Len(t, review.CodeComments, 1)
	assert.Equal(t, int64(4), review.CodeComments["README.md"][int64(4)][0].Line)
}

func TestReviewType_Icon(t *testing.T) {
	assert.Equal(t, "check", issues_model.ReviewTypeApprove.Icon())
	assert.Equal(t, "diff", issues_model.ReviewTypeReject.Icon())
	assert.Equal(t, "comment", issues_model.ReviewTypeComment.Icon())
	assert.Equal(t, "comment", issues_model.ReviewTypeUnknown.Icon())
	assert.Equal(t, "dot-fill", issues_model.ReviewTypeRequest.Icon())
	assert.Equal(t, "comment", issues_model.ReviewType(6).Icon())
}

func TestFindReviews(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	reviews, err := issues_model.FindReviews(t.Context(), issues_model.FindReviewOptions{
		Types:      []issues_model.ReviewType{issues_model.ReviewTypeApprove},
		IssueID:    2,
		ReviewerID: 1,
	})
	assert.NoError(t, err)
	assert.Len(t, reviews, 1)
	assert.Equal(t, "Demo Review", reviews[0].Content)
}

func TestFindLatestReviews(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	reviews, err := issues_model.FindLatestReviews(t.Context(), issues_model.FindReviewOptions{
		Types:   []issues_model.ReviewType{issues_model.ReviewTypeApprove},
		IssueID: 11,
	})
	assert.NoError(t, err)
	assert.Len(t, reviews, 2)
	assert.Equal(t, "duplicate review from user5 (latest)", reviews[0].Content)
	assert.Equal(t, "singular review from org6 and final review for this pr", reviews[1].Content)
}

func TestGetCurrentReview(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	review, err := issues_model.GetCurrentReview(t.Context(), user, issue)
	assert.NoError(t, err)
	assert.NotNil(t, review)
	assert.Equal(t, issues_model.ReviewTypePending, review.Type)
	assert.Equal(t, "Pending Review", review.Content)

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 7})
	review2, err := issues_model.GetCurrentReview(t.Context(), user2, issue)
	assert.Error(t, err)
	assert.True(t, issues_model.IsErrReviewNotExist(err))
	assert.Nil(t, review2)
}

func TestCreateReview(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	review, err := issues_model.CreateReview(t.Context(), issues_model.CreateReviewOptions{
		Content:  "New Review",
		Type:     issues_model.ReviewTypePending,
		Issue:    issue,
		Reviewer: user,
	})
	assert.NoError(t, err)
	assert.Equal(t, "New Review", review.Content)
	unittest.AssertExistsAndLoadBean(t, &issues_model.Review{Content: "New Review"})
}

func TestGetReviewersByIssueID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	expectedReviews := []*issues_model.Review{}
	expectedReviews = append(expectedReviews,
		&issues_model.Review{
			ID:          7,
			Reviewer:    org3,
			Type:        issues_model.ReviewTypeReject,
			UpdatedUnix: 946684812,
		},
		&issues_model.Review{
			ID:          8,
			Reviewer:    user4,
			Type:        issues_model.ReviewTypeApprove,
			UpdatedUnix: 946684813,
		},
		&issues_model.Review{
			ID:          9,
			Reviewer:    user2,
			Type:        issues_model.ReviewTypeReject,
			UpdatedUnix: 946684814,
		},
		&issues_model.Review{
			ID:          10,
			Reviewer:    user_model.NewGhostUser(),
			Type:        issues_model.ReviewTypeReject,
			UpdatedUnix: 946684815,
		},
		&issues_model.Review{
			ID:          22,
			Reviewer:    user5,
			Type:        issues_model.ReviewTypeRequest,
			UpdatedUnix: 946684817,
		},
	)

	allReviews, migratedReviews, err := issues_model.GetReviewsByIssueID(t.Context(), issue.ID)
	assert.NoError(t, err)
	assert.Empty(t, migratedReviews)
	for _, review := range allReviews {
		assert.NoError(t, review.LoadReviewer(t.Context()))
	}
	if assert.Len(t, allReviews, 5) {
		for i, review := range allReviews {
			assert.Equal(t, expectedReviews[i].Reviewer, review.Reviewer)
			assert.Equal(t, expectedReviews[i].Type, review.Type)
			assert.Equal(t, expectedReviews[i].UpdatedUnix, review.UpdatedUnix)
		}
	}
}

func TestDismissReview(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	rejectReviewExample := unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 9})
	requestReviewExample := unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 11})
	approveReviewExample := unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 8})
	assert.False(t, rejectReviewExample.Dismissed)
	assert.False(t, requestReviewExample.Dismissed)
	assert.False(t, approveReviewExample.Dismissed)

	assert.NoError(t, issues_model.DismissReview(t.Context(), rejectReviewExample, true))
	rejectReviewExample = unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 9})
	requestReviewExample = unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 11})
	assert.True(t, rejectReviewExample.Dismissed)
	assert.False(t, requestReviewExample.Dismissed)

	assert.NoError(t, issues_model.DismissReview(t.Context(), requestReviewExample, true))
	rejectReviewExample = unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 9})
	requestReviewExample = unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 11})
	assert.True(t, rejectReviewExample.Dismissed)
	assert.False(t, requestReviewExample.Dismissed)
	assert.False(t, approveReviewExample.Dismissed)

	assert.NoError(t, issues_model.DismissReview(t.Context(), requestReviewExample, true))
	rejectReviewExample = unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 9})
	requestReviewExample = unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 11})
	assert.True(t, rejectReviewExample.Dismissed)
	assert.False(t, requestReviewExample.Dismissed)
	assert.False(t, approveReviewExample.Dismissed)

	assert.NoError(t, issues_model.DismissReview(t.Context(), requestReviewExample, false))
	rejectReviewExample = unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 9})
	requestReviewExample = unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 11})
	assert.True(t, rejectReviewExample.Dismissed)
	assert.False(t, requestReviewExample.Dismissed)
	assert.False(t, approveReviewExample.Dismissed)

	assert.NoError(t, issues_model.DismissReview(t.Context(), requestReviewExample, false))
	rejectReviewExample = unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 9})
	requestReviewExample = unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 11})
	assert.True(t, rejectReviewExample.Dismissed)
	assert.False(t, requestReviewExample.Dismissed)
	assert.False(t, approveReviewExample.Dismissed)

	assert.NoError(t, issues_model.DismissReview(t.Context(), rejectReviewExample, false))
	assert.False(t, rejectReviewExample.Dismissed)
	assert.False(t, requestReviewExample.Dismissed)
	assert.False(t, approveReviewExample.Dismissed)

	assert.NoError(t, issues_model.DismissReview(t.Context(), approveReviewExample, true))
	assert.False(t, rejectReviewExample.Dismissed)
	assert.False(t, requestReviewExample.Dismissed)
	assert.True(t, approveReviewExample.Dismissed)
}

func TestDeleteReview(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	review1, err := issues_model.CreateReview(t.Context(), issues_model.CreateReviewOptions{
		Content:  "Official rejection",
		Type:     issues_model.ReviewTypeReject,
		Official: false,
		Issue:    issue,
		Reviewer: user,
	})
	assert.NoError(t, err)

	review2, err := issues_model.CreateReview(t.Context(), issues_model.CreateReviewOptions{
		Content:  "Official approval",
		Type:     issues_model.ReviewTypeApprove,
		Official: true,
		Issue:    issue,
		Reviewer: user,
	})
	assert.NoError(t, err)

	assert.NoError(t, issues_model.DeleteReview(t.Context(), review2))

	_, err = issues_model.GetReviewByID(t.Context(), review2.ID)
	assert.Error(t, err)
	assert.True(t, issues_model.IsErrReviewNotExist(err), "IsErrReviewNotExist")

	review1, err = issues_model.GetReviewByID(t.Context(), review1.ID)
	assert.NoError(t, err)
	assert.True(t, review1.Official)
}

func TestDeleteDismissedReview(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})
	review, err := issues_model.CreateReview(t.Context(), issues_model.CreateReviewOptions{
		Content:  "reject",
		Type:     issues_model.ReviewTypeReject,
		Official: false,
		Issue:    issue,
		Reviewer: user,
	})
	assert.NoError(t, err)
	assert.NoError(t, issues_model.DismissReview(t.Context(), review, true))
	comment, err := issues_model.CreateComment(t.Context(), &issues_model.CreateCommentOptions{
		Type:     issues_model.CommentTypeDismissReview,
		Doer:     user,
		Repo:     repo,
		Issue:    issue,
		ReviewID: review.ID,
		Content:  "dismiss",
	})
	assert.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: comment.ID})
	assert.NoError(t, issues_model.DeleteReview(t.Context(), review))
	unittest.AssertNotExistsBean(t, &issues_model.Comment{ID: comment.ID})
}

func TestAddReviewRequest(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pull := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1})
	assert.NoError(t, pull.LoadIssue(t.Context()))
	issue := pull.Issue
	assert.NoError(t, issue.LoadRepo(t.Context()))
	reviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	_, err := issues_model.CreateReview(t.Context(), issues_model.CreateReviewOptions{
		Issue:    issue,
		Reviewer: reviewer,
		Type:     issues_model.ReviewTypeReject,
	})

	assert.NoError(t, err)
	pull.HasMerged = false
	assert.NoError(t, pull.UpdateCols(t.Context(), "has_merged"))
	issue.IsClosed = true
	_, err = issues_model.AddReviewRequest(t.Context(), issue, reviewer, &user_model.User{})
	assert.Error(t, err)
	assert.True(t, issues_model.IsErrReviewRequestOnClosedPR(err))

	pull.HasMerged = true
	assert.NoError(t, pull.UpdateCols(t.Context(), "has_merged"))
	issue.IsClosed = false
	_, err = issues_model.AddReviewRequest(t.Context(), issue, reviewer, &user_model.User{})
	assert.Error(t, err)
	assert.True(t, issues_model.IsErrReviewRequestOnClosedPR(err))
}
