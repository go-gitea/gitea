// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestGetReviewByID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	review, err := issues_model.GetReviewByID(db.DefaultContext, 1)
	assert.NoError(t, err)
	assert.Equal(t, "Demo Review", review.Content)
	assert.Equal(t, issues_model.ReviewTypeApprove, review.Type)

	_, err = issues_model.GetReviewByID(db.DefaultContext, 23892)
	assert.Error(t, err)
	assert.True(t, issues_model.IsErrReviewNotExist(err), "IsErrReviewNotExist")
}

func TestReview_LoadAttributes(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	review := unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 1})
	assert.NoError(t, review.LoadAttributes(db.DefaultContext))
	assert.NotNil(t, review.Issue)
	assert.NotNil(t, review.Reviewer)

	invalidReview1 := unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 2})
	assert.Error(t, invalidReview1.LoadAttributes(db.DefaultContext))

	invalidReview2 := unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 3})
	assert.Error(t, invalidReview2.LoadAttributes(db.DefaultContext))
}

func TestReview_LoadCodeComments(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	review := unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 4})
	assert.NoError(t, review.LoadAttributes(db.DefaultContext))
	assert.NoError(t, review.LoadCodeComments(db.DefaultContext))
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
	reviews, err := issues_model.FindReviews(db.DefaultContext, issues_model.FindReviewOptions{
		Type:       issues_model.ReviewTypeApprove,
		IssueID:    2,
		ReviewerID: 1,
	})
	assert.NoError(t, err)
	assert.Len(t, reviews, 1)
	assert.Equal(t, "Demo Review", reviews[0].Content)
}

func TestFindLatestReviews(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	reviews, err := issues_model.FindLatestReviews(db.DefaultContext, issues_model.FindReviewOptions{
		Type:    issues_model.ReviewTypeApprove,
		IssueID: 11,
	})
	assert.NoError(t, err)
	assert.Len(t, reviews, 2)
	assert.Equal(t, "duplicate review from user5 (latest)", reviews[0].Content)
	assert.Equal(t, "singular review from user6 and final review for this pr", reviews[1].Content)
}

func TestGetCurrentReview(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	review, err := issues_model.GetCurrentReview(db.DefaultContext, user, issue)
	assert.NoError(t, err)
	assert.NotNil(t, review)
	assert.Equal(t, issues_model.ReviewTypePending, review.Type)
	assert.Equal(t, "Pending Review", review.Content)

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 7})
	review2, err := issues_model.GetCurrentReview(db.DefaultContext, user2, issue)
	assert.Error(t, err)
	assert.True(t, issues_model.IsErrReviewNotExist(err))
	assert.Nil(t, review2)
}

func TestCreateReview(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	review, err := issues_model.CreateReview(db.DefaultContext, issues_model.CreateReviewOptions{
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
	user3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	expectedReviews := []*issues_model.Review{}
	expectedReviews = append(expectedReviews,
		&issues_model.Review{
			Reviewer:    user3,
			Type:        issues_model.ReviewTypeReject,
			UpdatedUnix: 946684812,
		},
		&issues_model.Review{
			Reviewer:    user4,
			Type:        issues_model.ReviewTypeApprove,
			UpdatedUnix: 946684813,
		},
		&issues_model.Review{
			Reviewer:    user2,
			Type:        issues_model.ReviewTypeReject,
			UpdatedUnix: 946684814,
		})

	allReviews, err := issues_model.GetReviewsByIssueID(issue.ID)
	assert.NoError(t, err)
	for _, review := range allReviews {
		assert.NoError(t, review.LoadReviewer(db.DefaultContext))
	}
	if assert.Len(t, allReviews, 3) {
		for i, review := range allReviews {
			assert.Equal(t, expectedReviews[i].Reviewer, review.Reviewer)
			assert.Equal(t, expectedReviews[i].Type, review.Type)
			assert.Equal(t, expectedReviews[i].UpdatedUnix, review.UpdatedUnix)
		}
	}

	allReviews, err = issues_model.GetReviewsByIssueID(issue.ID)
	assert.NoError(t, err)
	assert.NoError(t, issues_model.LoadReviewers(db.DefaultContext, allReviews))
	if assert.Len(t, allReviews, 3) {
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

	assert.NoError(t, issues_model.DismissReview(rejectReviewExample, true))
	rejectReviewExample = unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 9})
	requestReviewExample = unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 11})
	assert.True(t, rejectReviewExample.Dismissed)
	assert.False(t, requestReviewExample.Dismissed)

	assert.NoError(t, issues_model.DismissReview(requestReviewExample, true))
	rejectReviewExample = unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 9})
	requestReviewExample = unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 11})
	assert.True(t, rejectReviewExample.Dismissed)
	assert.False(t, requestReviewExample.Dismissed)
	assert.False(t, approveReviewExample.Dismissed)

	assert.NoError(t, issues_model.DismissReview(requestReviewExample, true))
	rejectReviewExample = unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 9})
	requestReviewExample = unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 11})
	assert.True(t, rejectReviewExample.Dismissed)
	assert.False(t, requestReviewExample.Dismissed)
	assert.False(t, approveReviewExample.Dismissed)

	assert.NoError(t, issues_model.DismissReview(requestReviewExample, false))
	rejectReviewExample = unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 9})
	requestReviewExample = unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 11})
	assert.True(t, rejectReviewExample.Dismissed)
	assert.False(t, requestReviewExample.Dismissed)
	assert.False(t, approveReviewExample.Dismissed)

	assert.NoError(t, issues_model.DismissReview(requestReviewExample, false))
	rejectReviewExample = unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 9})
	requestReviewExample = unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 11})
	assert.True(t, rejectReviewExample.Dismissed)
	assert.False(t, requestReviewExample.Dismissed)
	assert.False(t, approveReviewExample.Dismissed)

	assert.NoError(t, issues_model.DismissReview(rejectReviewExample, false))
	assert.False(t, rejectReviewExample.Dismissed)
	assert.False(t, requestReviewExample.Dismissed)
	assert.False(t, approveReviewExample.Dismissed)

	assert.NoError(t, issues_model.DismissReview(approveReviewExample, true))
	assert.False(t, rejectReviewExample.Dismissed)
	assert.False(t, requestReviewExample.Dismissed)
	assert.True(t, approveReviewExample.Dismissed)
}

func TestDeleteReview(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	review1, err := issues_model.CreateReview(db.DefaultContext, issues_model.CreateReviewOptions{
		Content:  "Official rejection",
		Type:     issues_model.ReviewTypeReject,
		Official: false,
		Issue:    issue,
		Reviewer: user,
	})
	assert.NoError(t, err)

	review2, err := issues_model.CreateReview(db.DefaultContext, issues_model.CreateReviewOptions{
		Content:  "Official approval",
		Type:     issues_model.ReviewTypeApprove,
		Official: true,
		Issue:    issue,
		Reviewer: user,
	})
	assert.NoError(t, err)

	assert.NoError(t, issues_model.DeleteReview(review2))

	_, err = issues_model.GetReviewByID(db.DefaultContext, review2.ID)
	assert.Error(t, err)
	assert.True(t, issues_model.IsErrReviewNotExist(err), "IsErrReviewNotExist")

	review1, err = issues_model.GetReviewByID(db.DefaultContext, review1.ID)
	assert.NoError(t, err)
	assert.True(t, review1.Official)
}
