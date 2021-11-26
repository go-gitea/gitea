// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestGetReviewByID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	review, err := GetReviewByID(1)
	assert.NoError(t, err)
	assert.Equal(t, "Demo Review", review.Content)
	assert.Equal(t, ReviewTypeApprove, review.Type)

	_, err = GetReviewByID(23892)
	assert.Error(t, err)
	assert.True(t, IsErrReviewNotExist(err), "IsErrReviewNotExist")
}

func TestReview_LoadAttributes(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	review := unittest.AssertExistsAndLoadBean(t, &Review{ID: 1}).(*Review)
	assert.NoError(t, review.LoadAttributes())
	assert.NotNil(t, review.Issue)
	assert.NotNil(t, review.Reviewer)

	invalidReview1 := unittest.AssertExistsAndLoadBean(t, &Review{ID: 2}).(*Review)
	assert.Error(t, invalidReview1.LoadAttributes())

	invalidReview2 := unittest.AssertExistsAndLoadBean(t, &Review{ID: 3}).(*Review)
	assert.Error(t, invalidReview2.LoadAttributes())
}

func TestReview_LoadCodeComments(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	review := unittest.AssertExistsAndLoadBean(t, &Review{ID: 4}).(*Review)
	assert.NoError(t, review.LoadAttributes())
	assert.NoError(t, review.LoadCodeComments())
	assert.Len(t, review.CodeComments, 1)
	assert.Equal(t, int64(4), review.CodeComments["README.md"][int64(4)][0].Line)
}

func TestReviewType_Icon(t *testing.T) {
	assert.Equal(t, "check", ReviewTypeApprove.Icon())
	assert.Equal(t, "diff", ReviewTypeReject.Icon())
	assert.Equal(t, "comment", ReviewTypeComment.Icon())
	assert.Equal(t, "comment", ReviewTypeUnknown.Icon())
	assert.Equal(t, "dot-fill", ReviewTypeRequest.Icon())
	assert.Equal(t, "comment", ReviewType(6).Icon())
}

func TestFindReviews(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	reviews, err := FindReviews(FindReviewOptions{
		Type:       ReviewTypeApprove,
		IssueID:    2,
		ReviewerID: 1,
	})
	assert.NoError(t, err)
	assert.Len(t, reviews, 1)
	assert.Equal(t, "Demo Review", reviews[0].Content)
}

func TestGetCurrentReview(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &Issue{ID: 2}).(*Issue)
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)

	review, err := GetCurrentReview(user, issue)
	assert.NoError(t, err)
	assert.NotNil(t, review)
	assert.Equal(t, ReviewTypePending, review.Type)
	assert.Equal(t, "Pending Review", review.Content)

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 7}).(*user_model.User)
	review2, err := GetCurrentReview(user2, issue)
	assert.Error(t, err)
	assert.True(t, IsErrReviewNotExist(err))
	assert.Nil(t, review2)
}

func TestCreateReview(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &Issue{ID: 2}).(*Issue)
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)

	review, err := CreateReview(CreateReviewOptions{
		Content:  "New Review",
		Type:     ReviewTypePending,
		Issue:    issue,
		Reviewer: user,
	})
	assert.NoError(t, err)
	assert.Equal(t, "New Review", review.Content)
	unittest.AssertExistsAndLoadBean(t, &Review{Content: "New Review"})
}

func TestGetReviewersByIssueID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &Issue{ID: 3}).(*Issue)
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	user3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3}).(*user_model.User)
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4}).(*user_model.User)

	expectedReviews := []*Review{}
	expectedReviews = append(expectedReviews,
		&Review{
			Reviewer:    user3,
			Type:        ReviewTypeReject,
			UpdatedUnix: 946684812,
		},
		&Review{
			Reviewer:    user4,
			Type:        ReviewTypeApprove,
			UpdatedUnix: 946684813,
		},
		&Review{
			Reviewer:    user2,
			Type:        ReviewTypeReject,
			UpdatedUnix: 946684814,
		})

	allReviews, err := GetReviewersByIssueID(issue.ID)
	for _, reviewer := range allReviews {
		assert.NoError(t, reviewer.LoadReviewer())
	}
	assert.NoError(t, err)
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

	rejectReviewExample := unittest.AssertExistsAndLoadBean(t, &Review{ID: 9}).(*Review)
	requestReviewExample := unittest.AssertExistsAndLoadBean(t, &Review{ID: 11}).(*Review)
	approveReviewExample := unittest.AssertExistsAndLoadBean(t, &Review{ID: 8}).(*Review)
	assert.False(t, rejectReviewExample.Dismissed)
	assert.False(t, requestReviewExample.Dismissed)
	assert.False(t, approveReviewExample.Dismissed)

	assert.NoError(t, DismissReview(rejectReviewExample, true))
	rejectReviewExample = unittest.AssertExistsAndLoadBean(t, &Review{ID: 9}).(*Review)
	requestReviewExample = unittest.AssertExistsAndLoadBean(t, &Review{ID: 11}).(*Review)
	assert.True(t, rejectReviewExample.Dismissed)
	assert.False(t, requestReviewExample.Dismissed)

	assert.NoError(t, DismissReview(requestReviewExample, true))
	rejectReviewExample = unittest.AssertExistsAndLoadBean(t, &Review{ID: 9}).(*Review)
	requestReviewExample = unittest.AssertExistsAndLoadBean(t, &Review{ID: 11}).(*Review)
	assert.True(t, rejectReviewExample.Dismissed)
	assert.False(t, requestReviewExample.Dismissed)
	assert.False(t, approveReviewExample.Dismissed)

	assert.NoError(t, DismissReview(requestReviewExample, true))
	rejectReviewExample = unittest.AssertExistsAndLoadBean(t, &Review{ID: 9}).(*Review)
	requestReviewExample = unittest.AssertExistsAndLoadBean(t, &Review{ID: 11}).(*Review)
	assert.True(t, rejectReviewExample.Dismissed)
	assert.False(t, requestReviewExample.Dismissed)
	assert.False(t, approveReviewExample.Dismissed)

	assert.NoError(t, DismissReview(requestReviewExample, false))
	rejectReviewExample = unittest.AssertExistsAndLoadBean(t, &Review{ID: 9}).(*Review)
	requestReviewExample = unittest.AssertExistsAndLoadBean(t, &Review{ID: 11}).(*Review)
	assert.True(t, rejectReviewExample.Dismissed)
	assert.False(t, requestReviewExample.Dismissed)
	assert.False(t, approveReviewExample.Dismissed)

	assert.NoError(t, DismissReview(requestReviewExample, false))
	rejectReviewExample = unittest.AssertExistsAndLoadBean(t, &Review{ID: 9}).(*Review)
	requestReviewExample = unittest.AssertExistsAndLoadBean(t, &Review{ID: 11}).(*Review)
	assert.True(t, rejectReviewExample.Dismissed)
	assert.False(t, requestReviewExample.Dismissed)
	assert.False(t, approveReviewExample.Dismissed)

	assert.NoError(t, DismissReview(rejectReviewExample, false))
	assert.False(t, rejectReviewExample.Dismissed)
	assert.False(t, requestReviewExample.Dismissed)
	assert.False(t, approveReviewExample.Dismissed)

	assert.NoError(t, DismissReview(approveReviewExample, true))
	assert.False(t, rejectReviewExample.Dismissed)
	assert.False(t, requestReviewExample.Dismissed)
	assert.True(t, approveReviewExample.Dismissed)

}
