package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetReviewByID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	review, err := GetReviewByID(1)
	assert.NoError(t, err)
	assert.Equal(t, "Demo Review", review.Content)
	assert.Equal(t, ReviewTypeApprove, review.Type)

	_, err = GetReviewByID(23892)
	assert.Error(t, err)
	assert.True(t, IsErrReviewNotExist(err), "IsErrReviewNotExist")
}

func TestReview_LoadAttributes(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	review := AssertExistsAndLoadBean(t, &Review{ID: 1}).(*Review)
	assert.NoError(t, review.LoadAttributes())
	assert.NotNil(t, review.Issue)
	assert.NotNil(t, review.Reviewer)

	invalidReview1 := AssertExistsAndLoadBean(t, &Review{ID: 2}).(*Review)
	assert.Error(t, invalidReview1.LoadAttributes())

	invalidReview2 := AssertExistsAndLoadBean(t, &Review{ID: 3}).(*Review)
	assert.Error(t, invalidReview2.LoadAttributes())

}

func TestReview_LoadCodeComments(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	review := AssertExistsAndLoadBean(t, &Review{ID: 4}).(*Review)
	assert.NoError(t, review.LoadAttributes())
	assert.NoError(t, review.LoadCodeComments())
	assert.Len(t, review.CodeComments, 1)
	assert.Equal(t, int64(4), review.CodeComments["README.md"][int64(4)][0].Line)
}

func TestReviewType_Icon(t *testing.T) {
	assert.Equal(t, "eye", ReviewTypeApprove.Icon())
	assert.Equal(t, "x", ReviewTypeReject.Icon())
	assert.Equal(t, "comment", ReviewTypeComment.Icon())
	assert.Equal(t, "comment", ReviewTypeUnknown.Icon())
	assert.Equal(t, "comment", ReviewType(4).Icon())
}

func TestFindReviews(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
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
	assert.NoError(t, PrepareTestDatabase())
	issue := AssertExistsAndLoadBean(t, &Issue{ID: 2}).(*Issue)
	user := AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)

	review, err := GetCurrentReview(user, issue)
	assert.NoError(t, err)
	assert.NotNil(t, review)
	assert.Equal(t, ReviewTypePending, review.Type)
	assert.Equal(t, "Pending Review", review.Content)

	user2 := AssertExistsAndLoadBean(t, &User{ID: 7}).(*User)
	review2, err := GetCurrentReview(user2, issue)
	assert.Error(t, err)
	assert.True(t, IsErrReviewNotExist(err))
	assert.Nil(t, review2)
}

func TestCreateReview(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	issue := AssertExistsAndLoadBean(t, &Issue{ID: 2}).(*Issue)
	user := AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)

	review, err := CreateReview(CreateReviewOptions{
		Content:  "New Review",
		Type:     ReviewTypePending,
		Issue:    issue,
		Reviewer: user,
	})
	assert.NoError(t, err)
	assert.Equal(t, "New Review", review.Content)
	AssertExistsAndLoadBean(t, &Review{Content: "New Review"})
}

func TestGetReviewersByIssueID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	issue := AssertExistsAndLoadBean(t, &Issue{ID: 3}).(*Issue)
	user2 := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	user3 := AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	user4 := AssertExistsAndLoadBean(t, &User{ID: 4}).(*User)

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
	assert.NoError(t, err)
	for i, review := range allReviews {
		assert.Equal(t, expectedReviews[i].Reviewer, review.Reviewer)
		assert.Equal(t, expectedReviews[i].Type, review.Type)
		assert.Equal(t, expectedReviews[i].UpdatedUnix, review.UpdatedUnix)
	}
}
