// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/structs"
)

// ListPullReviews lists all reviews of a pull request
func ListPullReviews(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/pulls/{index}/reviews repository repoListPullReviews
	// ---
	// summary: List all reviews for a pull request.
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: index
	//   in: path
	//   description: index of the pull request
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/PullReviewList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	pr, err := models.GetPullRequestByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.NotFound("GetPullRequestByIndex", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return
	}

	if err = pr.LoadIssue(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadIssue", err)
		return
	}

	if err = pr.Issue.LoadRepo(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadIssue", err)
		return
	}

	allReviews, err := models.FindReviews(models.FindReviewOptions{
		Type:    models.ReviewTypeUnknown,
		IssueID: pr.IssueID,
	})

	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindReviews", err)
		return
	}

	var apiReviews []structs.PullRequestReview
	for _, review := range allReviews {
		// show pending reviews only for the user who created them
		if review.Type == models.ReviewTypePending && review.ReviewerID != ctx.User.ID {
			continue
		}

		if err = review.LoadReviewer(); err != nil {
			ctx.Error(http.StatusInternalServerError, "LoadReviewer", err)
			return
		}

		var reviewType string
		switch review.Type {
		case models.ReviewTypeApprove:
			reviewType = "APPROVE"
		case models.ReviewTypeReject:
			reviewType = "REJECT"
		case models.ReviewTypeComment:
			reviewType = "COMMENT"
		case models.ReviewTypePending:
			reviewType = "PENDING"
		default:
			reviewType = "UNKNOWN"

		}

		apiReviews = append(apiReviews, structs.PullRequestReview{
			ID:       review.ID,
			PRURL:    pr.Issue.APIURL(),
			Reviewer: review.Reviewer.APIFormat(),
			Body:     review.Content,
			Created:  review.CreatedUnix.AsTime(),
			CommitID: review.CommitID,
			Type:     reviewType,
		})
	}

	ctx.JSON(http.StatusOK, &apiReviews)
}

// GetPullReview gets a specific review of a pull request
func GetPullReview(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/pulls/{index}/reviews/{id} repository repoGetPullReview
	// ---
	// summary: Get a specific review for a pull request.
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: index
	//   in: path
	//   description: index of the pull request
	//   type: integer
	//   format: int64
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the review
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/PullReview"
	//   "404":
	//     "$ref": "#/responses/notFound"

	pr, err := models.GetPullRequestByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.NotFound("GetPullRequestByIndex", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return
	}

	review, err := models.GetReviewByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrReviewNotExist(err) {
			ctx.NotFound("GetReviewByID", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetReviewByID", err)
		}
		return
	}

	// validate the the review is for the given PR
	if review.IssueID != pr.IssueID {
		ctx.NotFound("ReviewNotInPR", err)
		return
	}

	// make sure that the user has access to this review if it is pending
	if review.Type == models.ReviewTypePending && review.ReviewerID != ctx.User.ID {
		ctx.NotFound("GetReviewByID", err)
		return
	}

	if err = pr.LoadIssue(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadIssue", err)
		return
	}

	if err = pr.Issue.LoadRepo(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadIssue", err)
		return
	}

	if err = review.LoadReviewer(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadReviewer", err)
		return
	}

	var reviewType string
	switch review.Type {
	case models.ReviewTypeApprove:
		reviewType = "APPROVE"
	case models.ReviewTypeReject:
		reviewType = "REJECT"
	case models.ReviewTypeComment:
		reviewType = "COMMENT"
	case models.ReviewTypePending:
		reviewType = "PENDING"
	default:
		reviewType = "UNKNOWN"

	}
	apiReview := structs.PullRequestReview{
		ID:       review.ID,
		PRURL:    pr.Issue.APIURL(),
		Reviewer: review.Reviewer.APIFormat(),
		Body:     review.Content,
		Created:  review.CreatedUnix.AsTime(),
		CommitID: review.CommitID,
		Type:     reviewType,
	}

	ctx.JSON(http.StatusOK, &apiReview)
}

// GetPullReviewComments lists all comments of a pull request review
func GetPullReviewComments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/pulls/{index}/reviews/{id}/comments repository repoGetPullReviewComments
	// ---
	// summary: Get a specific review for a pull request.
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: index
	//   in: path
	//   description: index of the pull request
	//   type: integer
	//   format: int64
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the review
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/PullReviewCommentList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	pr, err := models.GetPullRequestByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.NotFound("GetPullRequestByIndex", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return
	}

	review, err := models.GetReviewByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrReviewNotExist(err) {
			ctx.NotFound("GetReviewByID", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetReviewByID", err)
		}
		return
	}

	// validate the the review is for the given PR
	if review.IssueID != pr.IssueID {
		ctx.NotFound("ReviewNotInPR", err)
		return
	}

	// make sure that the user has access to this review if it is pending
	if review.Type == models.ReviewTypePending && review.ReviewerID != ctx.User.ID {
		ctx.NotFound("GetReviewByID", err)
		return
	}

	err = pr.LoadIssue()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadIssue", err)
		return
	}

	err = review.LoadAttributes()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
		return
	}

	err = review.LoadCodeComments()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadCodeComments", err)
		return
	}

	err = review.Issue.LoadRepo()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadRepo", err)
		return
	}

	var apiComments []structs.PullRequestReviewComment

	for _, lines := range review.CodeComments {
		for _, comments := range lines {
			for _, comment := range comments {
				apiComment := structs.PullRequestReviewComment{
					ID:       comment.ID,
					URL:      comment.HTMLURL(),
					PRURL:    review.Issue.APIURL(),
					ReviewID: review.ID,
					Path:     comment.TreePath,
					CommitID: comment.CommitSHA,
					DiffHunk: comment.Patch,
					Reviewer: review.Reviewer.APIFormat(),
					Body:     comment.Content,
				}
				if comment.Line < 0 {
					apiComment.OldLineNum = comment.UnsignedLine()
				} else {
					apiComment.LineNum = comment.UnsignedLine()
				}
				apiComments = append(apiComments, apiComment)
			}
		}
	}

	ctx.JSON(http.StatusOK, &apiComments)
}
