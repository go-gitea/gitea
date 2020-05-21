// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/utils"
	pull_service "code.gitea.io/gitea/services/pull"
)

// ListPullReviews lists all reviews of a pull request
func ListPullReviews(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/pulls/{index}/reviews repository repoListPullReviews
	// ---
	// summary: List all reviews for a pull request
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
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results, maximum page size is 50
	//   type: integer
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
		ctx.Error(http.StatusInternalServerError, "LoadRepo", err)
		return
	}

	allReviews, err := models.FindReviews(models.FindReviewOptions{
		ListOptions: utils.GetListOptions(ctx),
		Type:        models.ReviewTypeUnknown,
		IssueID:     pr.IssueID,
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
	// summary: Get a specific review for a pull request
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

	review, _, statusSet := prepareSingleReview(ctx)
	if statusSet {
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
	// summary: Get a specific review for a pull request
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

	review, _, statusSet := prepareSingleReview(ctx)
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

	review, _, statusSet := prepareSingleReview(ctx)
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

// CreatePullReview create a review to an pull request
func CreatePullReview(ctx *context.APIContext, opts api.CreatePullReviewOptions) {
	// swagger:operation POST /repos/{owner}/{repo}/pulls/{index}/reviews repository repoCreatePullReview
	// ---
	// summary: Create a review to an pull request
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
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/CreatePullReviewOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/PullReview"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	pr, err := models.GetPullRequestByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.NotFound("GetPullRequestByIndex", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return
	}

	// determine review type
	reviewType, isWrong := preparePullReviewType(ctx, pr, opts.Event, opts.Body)
	if isWrong {
		return
	}

	if err := pr.Issue.LoadRepo(); err != nil {
		ctx.Error(http.StatusInternalServerError, "pr.Issue.LoadRepo", err)
		return
	}

	// if CommitID is empty, set it as lastCommitID
	if opts.CommitID == "" {
		gitRepo, err := git.OpenRepository(pr.Issue.Repo.RepoPath())
		if err != nil {
			ctx.ServerError("git.OpenRepository", err)
			return
		}
		defer gitRepo.Close()

		headCommitID, err := gitRepo.GetRefCommitID(pr.GetGitRefName())
		if err != nil {
			ctx.ServerError("GetRefCommitID", err)
			return
		}

		opts.CommitID = headCommitID
	}

	// create review comments
	for _, c := range opts.Comments {
		line := c.NewLineNum
		if c.OldLineNum > 0 {
			line = c.OldLineNum * -1
		}

		if _, err := pull_service.CreateCodeComment(
			ctx.User,
			ctx.Repo.GitRepo,
			pr.Issue,
			line,
			c.Body,
			c.Path,
			true, // is review
			0,    // no reply
			opts.CommitID,
		); err != nil {
			ctx.ServerError("CreateCodeComment", err)
			return
		}
	}

	// create review and associate all pending review comments
	review, _, err := pull_service.SubmitReview(ctx.User, ctx.Repo.GitRepo, pr.Issue, reviewType, opts.Body, opts.CommitID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "SubmitReview", err)
		return
	}

	// convert response
	apiReview, err := convert.ToPullReview(review, ctx.User)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "convertToPullReview", err)
		return
	}
	ctx.JSON(http.StatusOK, apiReview)
}

// SubmitPullReview submit a pending review to an pull request
func SubmitPullReview(ctx *context.APIContext, opts api.SubmitPullReviewOptions) {
	// swagger:operation POST /repos/{owner}/{repo}/pulls/{index}/reviews/{id} repository repoSubmitPullReview
	// ---
	// summary: Submit a pending review to an pull request
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
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/SubmitPullReviewOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/PullReview"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	review, pr, isWrong := prepareSingleReview(ctx)
	if isWrong {
		return
	}

	if review.Type != models.ReviewTypePending {
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("only a pending review can be submitted"))
		return
	}

	// determine review type
	reviewType, isWrong := preparePullReviewType(ctx, pr, opts.Event, opts.Body)
	if isWrong {
		return
	}

	// if review stay pending return
	if reviewType == models.ReviewTypePending {
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("review stay pending"))
		return
	}

	headCommitID, err := ctx.Repo.GitRepo.GetRefCommitID(pr.GetGitRefName())
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GitRepo: GetRefCommitID", err)
		return
	}

	// create review and associate all pending review comments
	review, _, err = pull_service.SubmitReview(ctx.User, ctx.Repo.GitRepo, pr.Issue, reviewType, opts.Body, headCommitID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "SubmitReview", err)
		return
	}

	// convert response
	apiReview, err := convert.ToPullReview(review, ctx.User)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "convertToPullReview", err)
		return
	}
	ctx.JSON(http.StatusOK, apiReview)
}

// preparePullReviewType return ReviewType and false or nil and true if an error happen
func preparePullReviewType(ctx *context.APIContext, pr *models.PullRequest, event api.ReviewStateType, body string) (models.ReviewType, bool) {
	if err := pr.LoadIssue(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadIssue", err)
		return -1, true
	}

	var reviewType models.ReviewType
	switch event {
	case api.ReviewStateApproved:
		// can not approve your own PR
		if pr.Issue.IsPoster(ctx.User.ID) {
			ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("approve your own pull is not allowed"))
			return -1, true
		}
		reviewType = models.ReviewTypeApprove

	case api.ReviewStateRequestChanges:
		// can not reject your own PR
		if pr.Issue.IsPoster(ctx.User.ID) {
			ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("reject your own pull is not allowed"))
			return -1, true
		}
		reviewType = models.ReviewTypeReject

	case api.ReviewStateComment:
		reviewType = models.ReviewTypeComment
	default:
		reviewType = models.ReviewTypePending
	}

	// reject reviews with empty body if not approve type
	if reviewType != models.ReviewTypeApprove && len(strings.TrimSpace(body)) == 0 {
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("review event %s need body", event))
		return -1, true
	}

	return reviewType, false
}

// prepareSingleReview return review, related pull and false or nil, nil and true if an error happen
func prepareSingleReview(ctx *context.APIContext) (*models.Review, *models.PullRequest, bool) {
	pr, err := models.GetPullRequestByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.NotFound("GetPullRequestByIndex", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return nil, nil, true
	}

	review, err := models.GetReviewByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrReviewNotExist(err) {
			ctx.NotFound("GetReviewByID", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetReviewByID", err)
		}
		return nil, nil, true
	}

	// validate the the review is for the given PR
	if review.IssueID != pr.IssueID {
		ctx.NotFound("ReviewNotInPR")
		return nil, nil, true
	}

	// make sure that the user has access to this review if it is pending
	if review.Type == models.ReviewTypePending && review.ReviewerID != ctx.User.ID && !ctx.User.IsAdmin {
		ctx.NotFound("GetReviewByID")
		return nil, nil, true
	}

	if err := review.LoadAttributes(); err != nil && !models.IsErrUserNotExist(err) {
		ctx.Error(http.StatusInternalServerError, "ReviewLoadAttributes", err)
		return nil, nil, true
	}

	return review, pr, false
}
