package models

import (
	"testing"

	"fmt"
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

	user2 := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
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

func TestUpdateReview(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	review := AssertExistsAndLoadBean(t, &Review{ID: 1}).(*Review)
	review.Content = "Updated Review"
	assert.NoError(t, UpdateReview(review))
	AssertExistsAndLoadBean(t, &Review{ID: 1, Content: "Updated Review"})
}

func TestGetReviewersByPullID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	issue := AssertExistsAndLoadBean(t, &Issue{ID: 2}).(*Issue)
	user1 := AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)
	user2 := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	user3 := AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	user4 := AssertExistsAndLoadBean(t, &User{ID: 4}).(*User)

	// Create some reviews
	expectedReviews := make(map[int64]*PullReviewersWithType)
	reviews := []CreateReviewOptions{
		{
			Content:  "New Review 1",
			Type:     ReviewTypeComment,
			Issue:    issue,
			Reviewer: user1,
		},
		{
			Content:  "New Review 3",
			Type:     ReviewTypePending,
			Issue:    issue,
			Reviewer: user2,
		},
		{
			Content:  "New Review 4",
			Type:     ReviewTypeReject,
			Issue:    issue,
			Reviewer: user3,
		},
		{
			Content:  "New Review 5",
			Type:     ReviewTypeApprove,
			Issue:    issue,
			Reviewer: user4,
		},
	}
	for _, test := range reviews {
		rev, err := CreateReview(test)
		assert.NoError(t, err)
		// Only look for non-pending reviews
		if test.Type > 0 {
			expectedReviews[test.Reviewer.ID] = &PullReviewersWithType{
				User:              *test.Reviewer,
				Type:              test.Type,
				ReviewUpdatedUnix: rev.UpdatedUnix,
			}
		}
	}

	allReviews, err := GetReviewersByPullID(issue.ID)
	assert.NoError(t, err)
	assert.Equal(t, expectedReviews, allReviews)

	// Add another one, this time overwriting a previously "pending" one
	newReview := CreateReviewOptions{
		Content:  "New Review 56",
		Type:     ReviewTypeReject,
		Issue:    issue,
		Reviewer: user2,
	}
	rev, err := CreateReview(newReview)
	assert.NoError(t, err)
	fmt.Println(rev.Reviewer.ID)
	fmt.Println(newReview.Reviewer.ID)
	expectedReviews[newReview.Reviewer.ID] = &PullReviewersWithType{
		User:              *rev.Reviewer,
		Type:              rev.Type,
		ReviewUpdatedUnix: rev.UpdatedUnix,
	}

	// Check it
	allReviews, err = GetReviewersByPullID(issue.ID)
	assert.NoError(t, err)
	assert.Equal(t, expectedReviews, allReviews)
}
