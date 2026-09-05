// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestReview_HTMLURL(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	review := unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 1})
	assert.Equal(t, setting.AppURL+"user2/repo1/pulls/2#pullrequestreview-1", review.HTMLURL(t.Context()))

	pendingReview := unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 4})
	assert.Empty(t, pendingReview.HTMLURL(t.Context()))
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
	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	expectedReviews := []*issues_model.Review{}
	expectedReviews = append(expectedReviews,
		&issues_model.Review{
			ID:          5,
			Reviewer:    user1,
			Type:        issues_model.ReviewTypeComment,
			UpdatedUnix: 946684810,
		},
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
	if assert.Len(t, allReviews, 6) {
		for i, review := range allReviews {
			assert.Equal(t, expectedReviews[i].ID, review.ID)
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

func TestSubmitReviewClearsStaleReviewRequest(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3})
	assert.NoError(t, issue.LoadRepo(t.Context()))
	assert.NoError(t, issue.Repo.LoadOwner(t.Context()))
	reviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// the reviewer is requested to review the pull request
	requestReview, err := issues_model.CreateReview(t.Context(), issues_model.CreateReviewOptions{
		Type:     issues_model.ReviewTypeRequest,
		Issue:    issue,
		Reviewer: reviewer,
	})
	assert.NoError(t, err)

	// the reviewer starts a pending review (e.g. by adding code comments)
	pendingReview, err := issues_model.CreateReview(t.Context(), issues_model.CreateReviewOptions{
		Type:     issues_model.ReviewTypePending,
		Issue:    issue,
		Reviewer: reviewer,
	})
	assert.NoError(t, err)

	// submitting the pending review must clear the leftover review request,
	// otherwise the reviewer can no longer be re-requested afterwards
	review, _, err := issues_model.SubmitReview(t.Context(), reviewer, issue, issues_model.ReviewTypeComment, "looks good", "", false, nil)
	assert.NoError(t, err)
	assert.Equal(t, pendingReview.ID, review.ID)
	assert.Equal(t, issues_model.ReviewTypeComment, review.Type)

	unittest.AssertNotExistsBean(t, &issues_model.Review{ID: requestReview.ID})

	// the reviewer can be re-requested afterwards (no-op before the fix)
	comment, err := issues_model.AddReviewRequest(t.Context(), issue, reviewer, doer, false)
	assert.NoError(t, err)
	assert.NotNil(t, comment)
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
	_, err = issues_model.AddReviewRequest(t.Context(), issue, reviewer, &user_model.User{}, false)
	assert.Error(t, err)
	assert.True(t, issues_model.IsErrReviewRequestOnClosedPR(err))

	pull.HasMerged = true
	assert.NoError(t, pull.UpdateCols(t.Context(), "has_merged"))
	issue.IsClosed = false
	_, err = issues_model.AddReviewRequest(t.Context(), issue, reviewer, &user_model.User{}, false)
	assert.Error(t, err)
	assert.True(t, issues_model.IsErrReviewRequestOnClosedPR(err))

	// Test CODEOWNERS review request stores metadata correctly
	pull2 := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pull2.LoadIssue(t.Context()))
	issue2 := pull2.Issue
	assert.NoError(t, issue2.LoadRepo(t.Context()))
	reviewer2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 7})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	comment, err := issues_model.AddReviewRequest(t.Context(), issue2, reviewer2, doer, true)
	assert.NoError(t, err)
	assert.NotNil(t, comment)
	assert.NotNil(t, comment.CommentMetaData)
	assert.Equal(t, issues_model.SpecialDoerNameCodeOwners, comment.CommentMetaData.SpecialDoerName)
}

func TestAddReviewRequestIsIdempotentWhenAlreadyRequested(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pull := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pull.LoadIssue(t.Context()))
	issue := pull.Issue
	assert.NoError(t, issue.LoadRepo(t.Context()))
	reviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	requestReview, err := issues_model.CreateReview(t.Context(), issues_model.CreateReviewOptions{
		Type:     issues_model.ReviewTypeRequest,
		Issue:    issue,
		Reviewer: reviewer,
	})
	assert.NoError(t, err)

	comment, err := issues_model.AddReviewRequest(t.Context(), issue, reviewer, doer, false)
	assert.NoError(t, err)
	assert.Nil(t, comment)

	unittest.AssertCount(t, &issues_model.Review{IssueID: issue.ID, ReviewerID: reviewer.ID}, 1)
	unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: requestReview.ID})
}

func TestAddReviewRequestRecoversFromStaleRequestAlongsideNewerComment(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pull := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pull.LoadIssue(t.Context()))
	issue := pull.Issue
	assert.NoError(t, issue.LoadRepo(t.Context()))
	reviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 9})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// reproduce legacy data (e.g. imported via InsertReviews) where an old review
	// request row survives alongside a newer comment row for the same reviewer,
	// bypassing the usual CreateReview cleanup of stale request rows
	staleRequest := &issues_model.Review{Type: issues_model.ReviewTypeRequest, IssueID: issue.ID, ReviewerID: reviewer.ID}
	_, err := db.GetEngine(t.Context()).Insert(staleRequest)
	assert.NoError(t, err)

	newerComment := &issues_model.Review{Type: issues_model.ReviewTypeComment, IssueID: issue.ID, ReviewerID: reviewer.ID, Content: "looks good"}
	_, err = db.GetEngine(t.Context()).Insert(newerComment)
	assert.NoError(t, err)

	comment, err := issues_model.AddReviewRequest(t.Context(), issue, reviewer, doer, false)
	assert.NoError(t, err)
	require.NotNil(t, comment)
	assert.Equal(t, issues_model.CommentTypeReviewRequest, comment.Type)

	unittest.AssertNotExistsBean(t, &issues_model.Review{ID: staleRequest.ID})
	unittest.AssertCount(t, &issues_model.Review{IssueID: issue.ID, ReviewerID: reviewer.ID, Type: issues_model.ReviewTypeRequest}, 1)
	newRequest := unittest.AssertExistsAndLoadBean(t, &issues_model.Review{IssueID: issue.ID, ReviewerID: reviewer.ID, Type: issues_model.ReviewTypeRequest})
	assert.Equal(t, newRequest.ID, comment.ReviewID)
}

func TestAddReviewRequestAfterCommentOnly(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pull := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pull.LoadIssue(t.Context()))
	issue := pull.Issue
	assert.NoError(t, issue.LoadRepo(t.Context()))
	reviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 10})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	_, err := issues_model.CreateReview(t.Context(), issues_model.CreateReviewOptions{
		Type:     issues_model.ReviewTypeComment,
		Issue:    issue,
		Reviewer: reviewer,
		Content:  "just a comment",
	})
	assert.NoError(t, err)

	comment, err := issues_model.AddReviewRequest(t.Context(), issue, reviewer, doer, false)
	assert.NoError(t, err)
	assert.NotNil(t, comment)

	unittest.AssertCount(t, &issues_model.Review{IssueID: issue.ID, ReviewerID: reviewer.ID, Type: issues_model.ReviewTypeRequest}, 1)
}

func TestAddReviewRequestIsIdempotentAfterApproveThenRequest(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pull := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pull.LoadIssue(t.Context()))
	issue := pull.Issue
	assert.NoError(t, issue.LoadRepo(t.Context()))
	reviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 11})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	_, err := issues_model.CreateReview(t.Context(), issues_model.CreateReviewOptions{
		Type:     issues_model.ReviewTypeApprove,
		Issue:    issue,
		Reviewer: reviewer,
	})
	assert.NoError(t, err)

	requestReview, err := issues_model.CreateReview(t.Context(), issues_model.CreateReviewOptions{
		Type:     issues_model.ReviewTypeRequest,
		Issue:    issue,
		Reviewer: reviewer,
	})
	assert.NoError(t, err)

	comment, err := issues_model.AddReviewRequest(t.Context(), issue, reviewer, doer, false)
	assert.NoError(t, err)
	assert.Nil(t, comment)

	unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: requestReview.ID})
}

func TestAddReviewRequestPreservesClosedPRGuardScope(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pull := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pull.LoadIssue(t.Context()))
	issue := pull.Issue
	assert.NoError(t, issue.LoadRepo(t.Context()))
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	issue.IsClosed = true
	assert.NoError(t, issues_model.UpdateIssueCols(t.Context(), issue, "is_closed"))

	// The closed/merged guard only ever fired when the reviewer already had an
	// approve/reject/request review; a reviewer whose only history is a comment
	// never triggered it, closed PR or not. That pre-existing scope must not widen.
	reviewerWithCommentOnly := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 12})
	_, err := issues_model.CreateReview(t.Context(), issues_model.CreateReviewOptions{
		Type:     issues_model.ReviewTypeComment,
		Issue:    issue,
		Reviewer: reviewerWithCommentOnly,
		Content:  "just a comment",
	})
	assert.NoError(t, err)

	comment, err := issues_model.AddReviewRequest(t.Context(), issue, reviewerWithCommentOnly, doer, false)
	assert.NoError(t, err)
	assert.NotNil(t, comment)

	// A reviewer with a stale request row still has an approve/reject/request review
	// on record, so the guard must still reject re-requesting on the closed PR.
	reviewerWithStaleRequest := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 13})
	staleRequest := &issues_model.Review{Type: issues_model.ReviewTypeRequest, IssueID: issue.ID, ReviewerID: reviewerWithStaleRequest.ID}
	_, err = db.GetEngine(t.Context()).Insert(staleRequest)
	assert.NoError(t, err)

	newerComment := &issues_model.Review{Type: issues_model.ReviewTypeComment, IssueID: issue.ID, ReviewerID: reviewerWithStaleRequest.ID, Content: "looks good"}
	_, err = db.GetEngine(t.Context()).Insert(newerComment)
	assert.NoError(t, err)

	_, err = issues_model.AddReviewRequest(t.Context(), issue, reviewerWithStaleRequest, doer, false)
	assert.Error(t, err)
	assert.True(t, issues_model.IsErrReviewRequestOnClosedPR(err))
}

func TestRecalculateReviewsOfficial(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// PR #2 targets repo1's "master" branch. Simulate an approval that became
	// official while the PR targeted an unprotected branch.
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3})
	reviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	review, err := issues_model.CreateReview(t.Context(), issues_model.CreateReviewOptions{
		Type:     issues_model.ReviewTypeApprove,
		Issue:    issue,
		Reviewer: reviewer,
		Official: true,
	})
	assert.NoError(t, err)

	// Protect the (now current) target branch with an approvals whitelist that
	// does not include the reviewer, mirroring a retarget onto a protected branch.
	rule := &git_model.ProtectedBranch{
		RepoID:                    issue.RepoID,
		RuleName:                  "master",
		EnableApprovalsWhitelist:  true,
		ApprovalsWhitelistUserIDs: []int64{2},
		RequiredApprovals:         1,
	}
	assert.NoError(t, db.Insert(t.Context(), rule))

	// Re-evaluating must strip the stale official flag, otherwise the approval
	// would still satisfy the protected branch's required approvals.
	assert.NoError(t, issues_model.RecalculateReviewsOfficial(t.Context(), issue))
	review = unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: review.ID})
	assert.False(t, review.Official)

	// Once the reviewer is whitelisted, re-evaluating restores the official flag.
	rule.ApprovalsWhitelistUserIDs = []int64{2, reviewer.ID}
	_, err = db.GetEngine(t.Context()).ID(rule.ID).Cols("approvals_whitelist_user_i_ds").Update(rule)
	assert.NoError(t, err)

	assert.NoError(t, issues_model.RecalculateReviewsOfficial(t.Context(), issue))
	review = unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: review.ID})
	assert.True(t, review.Official)
}
