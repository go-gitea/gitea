// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
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

	apiReviews, err := convert.ToPullReviewList(allReviews, ctx.User)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "convertToPullReviewList", err)
		return
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
	if review.Type == models.ReviewTypePending && (ctx.User.IsAdmin || review.ReviewerID != ctx.User.ID) {
		ctx.NotFound("GetReviewByID", err)
		return
	}

	apiReview, err := convert.ToPullReview(review, ctx.User)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "convertToPullReview", err)
		return
	}

	ctx.JSON(http.StatusOK, apiReview)
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

	review, statusSet := prepareSingleReview(ctx)
	if statusSet {
		return
	}

	apiComments, err := convert.ToPullReviewCommentList(review, ctx.User)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "convertToPullReviewCommentList", err)
		return
	}

	ctx.JSON(http.StatusOK, apiComments)
}

// DeletePullReview delete a specific review from a pull request
func DeletePullReview(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/pulls/{index}/reviews/{id} repository repoDeletePullReview
	// ---
	// summary: Delete a specific review from a pull request
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
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	review, statusSet := prepareSingleReview(ctx)
	if statusSet {
		return
	}

	if ctx.User == nil {
		ctx.NotFound()
		return
	}
	if !ctx.User.IsAdmin && ctx.User.ID != review.ReviewerID {
		ctx.Error(http.StatusForbidden, "only admin and user itself can delete a review", nil)
		return
	}

	if err := models.DeleteReview(review); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteReview", fmt.Errorf("can not delete ReviewID: %d", review.ID))
		return
	}

	ctx.Status(http.StatusNoContent)
}

func prepareSingleReview(ctx *context.APIContext) (r *models.Review, statusSet bool) {
	pr, err := models.GetPullRequestByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.NotFound("GetPullRequestByIndex", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return nil, true
	}

	review, err := models.GetReviewByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrReviewNotExist(err) {
			ctx.NotFound("GetReviewByID", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetReviewByID", err)
		}
		return nil, true
	}

	// validate the the review is for the given PR
	if review.IssueID != pr.IssueID {
		ctx.NotFound("ReviewNotInPR", err)
		return nil, true
	}

	// make sure that the user has access to this review if it is pending
	if review.Type == models.ReviewTypePending && review.ReviewerID != ctx.User.ID {
		ctx.NotFound("GetReviewByID", err)
		return nil, true
	}

	return review, false
}
