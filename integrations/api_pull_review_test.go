// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
	"github.com/stretchr/testify/assert"
)

func TestAPIPullReview(t *testing.T) {
	defer prepareTestEnv(t)()
	pullIssue := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 3}).(*models.Issue)
	assert.NoError(t, pullIssue.LoadAttributes())
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: pullIssue.RepoID}).(*models.Repository)

	// test ListPullReviews
	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/reviews?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var reviews []*api.PullReview
	DecodeJSON(t, resp, &reviews)
	if !assert.Len(t, reviews, 6) {
		return
	}
	for _, r := range reviews {
		assert.EqualValues(t, pullIssue.HTMLURL(), r.HTMLPullURL)
	}
	assert.EqualValues(t, 8, reviews[3].ID)
	assert.EqualValues(t, "APPROVED", reviews[3].State)
	assert.EqualValues(t, 0, reviews[3].CodeCommentsCount)
	assert.EqualValues(t, true, reviews[3].Stale)
	assert.EqualValues(t, false, reviews[3].Official)

	assert.EqualValues(t, 10, reviews[5].ID)
	assert.EqualValues(t, "REQUEST_CHANGES", reviews[5].State)
	assert.EqualValues(t, 1, reviews[5].CodeCommentsCount)
	assert.EqualValues(t, -1, reviews[5].Reviewer.ID) // ghost user
	assert.EqualValues(t, false, reviews[5].Stale)
	assert.EqualValues(t, true, reviews[5].Official)

	// test GetPullReview
	req = NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/reviews/%d?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, reviews[3].ID, token)
	resp = session.MakeRequest(t, req, http.StatusOK)
	var review api.PullReview
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, *reviews[3], review)

	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/pulls/%d/reviews/%d?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, reviews[5].ID, token)
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, *reviews[5], review)

	// test GetPullReviewComments
	comment := models.AssertExistsAndLoadBean(t, &models.Comment{ID: 7}).(*models.Comment)
	req = NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/reviews/%d/comments?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, 10, token)
	resp = session.MakeRequest(t, req, http.StatusOK)
	var reviewComments []*api.PullReviewComment
	DecodeJSON(t, resp, &reviewComments)
	assert.Len(t, reviewComments, 1)
	assert.EqualValues(t, "Ghost", reviewComments[0].Reviewer.UserName)
	assert.EqualValues(t, "a review from a deleted user", reviewComments[0].Body)
	assert.EqualValues(t, comment.ID, reviewComments[0].ID)
	assert.EqualValues(t, comment.UpdatedUnix, reviewComments[0].Updated.Unix())
	assert.EqualValues(t, comment.HTMLURL(), reviewComments[0].HTMLURL)

	// test CreatePullReview
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token), &api.CreatePullReviewOptions{
		Body: "body1",
		// Event: "" # will result in PENDING
		Comments: []api.CreatePullReviewComment{{
			Path:       "README.md",
			Body:       "first new line",
			OldLineNum: 0,
			NewLineNum: 1,
		}, {
			Path:       "README.md",
			Body:       "first old line",
			OldLineNum: 1,
			NewLineNum: 0,
		}, {
			Path:       "iso-8859-1.txt",
			Body:       "this line contains a non-utf-8 character",
			OldLineNum: 0,
			NewLineNum: 1,
		},
		},
	})
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, 6, review.ID)
	assert.EqualValues(t, "PENDING", review.State)
	assert.EqualValues(t, 3, review.CodeCommentsCount)

	// test SubmitPullReview
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews/%d?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, review.ID, token), &api.SubmitPullReviewOptions{
		Event: "APPROVED",
		Body:  "just two nits",
	})
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, 6, review.ID)
	assert.EqualValues(t, "APPROVED", review.State)
	assert.EqualValues(t, 3, review.CodeCommentsCount)

	// test DeletePullReview
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token), &api.CreatePullReviewOptions{
		Body:  "just a comment",
		Event: "COMMENT",
	})
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, "COMMENT", review.State)
	assert.EqualValues(t, 0, review.CodeCommentsCount)
	req = NewRequestf(t, http.MethodDelete, "/api/v1/repos/%s/%s/pulls/%d/reviews/%d?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, review.ID, token)
	resp = session.MakeRequest(t, req, http.StatusNoContent)
}
