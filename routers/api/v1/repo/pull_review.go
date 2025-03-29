// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/gitrepo"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	issue_service "code.gitea.io/gitea/services/issue"
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
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/PullReviewList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	pr, err := issues_model.GetPullRequestByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrPullRequestNotExist(err) {
			ctx.APIErrorNotFound("GetPullRequestByIndex", err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if err = pr.LoadIssue(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	if err = pr.Issue.LoadRepo(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	opts := issues_model.FindReviewOptions{
		ListOptions: utils.GetListOptions(ctx),
		IssueID:     pr.IssueID,
	}

	allReviews, err := issues_model.FindReviews(ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	count, err := issues_model.CountReviews(ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiReviews, err := convert.ToPullReviewList(ctx, allReviews, ctx.Doer)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.SetTotalCountHeader(count)
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

	apiReview, err := convert.ToPullReview(ctx, review, ctx.Doer)
	if err != nil {
		ctx.APIErrorInternal(err)
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

	apiComments, err := convert.ToPullReviewCommentList(ctx, review, ctx.Doer)
	if err != nil {
		ctx.APIErrorInternal(err)
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

	if ctx.Doer == nil {
		ctx.APIErrorNotFound()
		return
	}
	if !ctx.Doer.IsAdmin && ctx.Doer.ID != review.ReviewerID {
		ctx.APIError(http.StatusForbidden, nil)
		return
	}

	if err := issues_model.DeleteReview(ctx, review); err != nil {
		ctx.APIErrorInternal(fmt.Errorf("can not delete ReviewID: %d", review.ID))
		return
	}

	ctx.Status(http.StatusNoContent)
}

// CreatePullReview create a review to a pull request
func CreatePullReview(ctx *context.APIContext) {
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

	opts := web.GetForm(ctx).(*api.CreatePullReviewOptions)
	pr, err := issues_model.GetPullRequestByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrPullRequestNotExist(err) {
			ctx.APIErrorNotFound("GetPullRequestByIndex", err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	// determine review type
	reviewType, isWrong := preparePullReviewType(ctx, pr, opts.Event, opts.Body, len(opts.Comments) > 0)
	if isWrong {
		return
	}

	if err := pr.Issue.LoadRepo(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	// if CommitID is empty, set it as lastCommitID
	if opts.CommitID == "" {
		gitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, pr.Issue.Repo)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		defer closer.Close()

		headCommitID, err := gitRepo.GetRefCommitID(pr.GetGitRefName())
		if err != nil {
			ctx.APIErrorInternal(err)
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

		if _, err := pull_service.CreateCodeComment(ctx,
			ctx.Doer,
			ctx.Repo.GitRepo,
			pr.Issue,
			line,
			c.Body,
			c.Path,
			true, // pending review
			0,    // no reply
			opts.CommitID,
			nil,
		); err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}

	// create review and associate all pending review comments
	review, _, err := pull_service.SubmitReview(ctx, ctx.Doer, ctx.Repo.GitRepo, pr.Issue, reviewType, opts.Body, opts.CommitID, nil)
	if err != nil {
		if errors.Is(err, pull_service.ErrSubmitReviewOnClosedPR) {
			ctx.APIError(http.StatusUnprocessableEntity, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	// convert response
	apiReview, err := convert.ToPullReview(ctx, review, ctx.Doer)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, apiReview)
}

// SubmitPullReview submit a pending review to an pull request
func SubmitPullReview(ctx *context.APIContext) {
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

	opts := web.GetForm(ctx).(*api.SubmitPullReviewOptions)
	review, pr, isWrong := prepareSingleReview(ctx)
	if isWrong {
		return
	}

	if review.Type != issues_model.ReviewTypePending {
		ctx.APIError(http.StatusUnprocessableEntity, fmt.Errorf("only a pending review can be submitted"))
		return
	}

	// determine review type
	reviewType, isWrong := preparePullReviewType(ctx, pr, opts.Event, opts.Body, len(review.Comments) > 0)
	if isWrong {
		return
	}

	// if review stay pending return
	if reviewType == issues_model.ReviewTypePending {
		ctx.APIError(http.StatusUnprocessableEntity, fmt.Errorf("review stay pending"))
		return
	}

	headCommitID, err := ctx.Repo.GitRepo.GetRefCommitID(pr.GetGitRefName())
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	// create review and associate all pending review comments
	review, _, err = pull_service.SubmitReview(ctx, ctx.Doer, ctx.Repo.GitRepo, pr.Issue, reviewType, opts.Body, headCommitID, nil)
	if err != nil {
		if errors.Is(err, pull_service.ErrSubmitReviewOnClosedPR) {
			ctx.APIError(http.StatusUnprocessableEntity, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	// convert response
	apiReview, err := convert.ToPullReview(ctx, review, ctx.Doer)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, apiReview)
}

// preparePullReviewType return ReviewType and false or nil and true if an error happen
func preparePullReviewType(ctx *context.APIContext, pr *issues_model.PullRequest, event api.ReviewStateType, body string, hasComments bool) (issues_model.ReviewType, bool) {
	if err := pr.LoadIssue(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return -1, true
	}

	needsBody := true
	hasBody := len(strings.TrimSpace(body)) > 0

	var reviewType issues_model.ReviewType
	switch event {
	case api.ReviewStateApproved:
		// can not approve your own PR
		if pr.Issue.IsPoster(ctx.Doer.ID) {
			ctx.APIError(http.StatusUnprocessableEntity, fmt.Errorf("approve your own pull is not allowed"))
			return -1, true
		}
		reviewType = issues_model.ReviewTypeApprove
		needsBody = false

	case api.ReviewStateRequestChanges:
		// can not reject your own PR
		if pr.Issue.IsPoster(ctx.Doer.ID) {
			ctx.APIError(http.StatusUnprocessableEntity, fmt.Errorf("reject your own pull is not allowed"))
			return -1, true
		}
		reviewType = issues_model.ReviewTypeReject

	case api.ReviewStateComment:
		reviewType = issues_model.ReviewTypeComment
		needsBody = false
		// if there is no body we need to ensure that there are comments
		if !hasBody && !hasComments {
			ctx.APIError(http.StatusUnprocessableEntity, fmt.Errorf("review event %s requires a body or a comment", event))
			return -1, true
		}
	default:
		reviewType = issues_model.ReviewTypePending
	}

	// reject reviews with empty body if a body is required for this call
	if needsBody && !hasBody {
		ctx.APIError(http.StatusUnprocessableEntity, fmt.Errorf("review event %s requires a body", event))
		return -1, true
	}

	return reviewType, false
}

// prepareSingleReview return review, related pull and false or nil, nil and true if an error happen
func prepareSingleReview(ctx *context.APIContext) (*issues_model.Review, *issues_model.PullRequest, bool) {
	pr, err := issues_model.GetPullRequestByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrPullRequestNotExist(err) {
			ctx.APIErrorNotFound("GetPullRequestByIndex", err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return nil, nil, true
	}

	review, err := issues_model.GetReviewByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		if issues_model.IsErrReviewNotExist(err) {
			ctx.APIErrorNotFound("GetReviewByID", err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return nil, nil, true
	}

	// validate the review is for the given PR
	if review.IssueID != pr.IssueID {
		ctx.APIErrorNotFound("ReviewNotInPR")
		return nil, nil, true
	}

	// make sure that the user has access to this review if it is pending
	if review.Type == issues_model.ReviewTypePending && review.ReviewerID != ctx.Doer.ID && !ctx.Doer.IsAdmin {
		ctx.APIErrorNotFound("GetReviewByID")
		return nil, nil, true
	}

	if err := review.LoadAttributes(ctx); err != nil && !user_model.IsErrUserNotExist(err) {
		ctx.APIErrorInternal(err)
		return nil, nil, true
	}

	return review, pr, false
}

// CreateReviewRequests create review requests to an pull request
func CreateReviewRequests(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/pulls/{index}/requested_reviewers repository repoCreatePullReviewRequests
	// ---
	// summary: create review requests for a pull request
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
	//     "$ref": "#/definitions/PullReviewRequestOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/PullReviewList"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	opts := web.GetForm(ctx).(*api.PullReviewRequestOptions)
	apiReviewRequest(ctx, *opts, true)
}

// DeleteReviewRequests delete review requests to an pull request
func DeleteReviewRequests(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/pulls/{index}/requested_reviewers repository repoDeletePullReviewRequests
	// ---
	// summary: cancel review requests for a pull request
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
	//     "$ref": "#/definitions/PullReviewRequestOptions"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	opts := web.GetForm(ctx).(*api.PullReviewRequestOptions)
	apiReviewRequest(ctx, *opts, false)
}

func parseReviewersByNames(ctx *context.APIContext, reviewerNames, teamReviewerNames []string) (reviewers []*user_model.User, teamReviewers []*organization.Team) {
	var err error
	for _, r := range reviewerNames {
		var reviewer *user_model.User
		if strings.Contains(r, "@") {
			reviewer, err = user_model.GetUserByEmail(ctx, r)
		} else {
			reviewer, err = user_model.GetUserByName(ctx, r)
		}

		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				ctx.APIErrorNotFound("UserNotExist", fmt.Sprintf("User '%s' not exist", r))
				return nil, nil
			}
			ctx.APIErrorInternal(err)
			return nil, nil
		}

		reviewers = append(reviewers, reviewer)
	}

	if ctx.Repo.Repository.Owner.IsOrganization() && len(teamReviewerNames) > 0 {
		for _, t := range teamReviewerNames {
			var teamReviewer *organization.Team
			teamReviewer, err = organization.GetTeam(ctx, ctx.Repo.Owner.ID, t)
			if err != nil {
				if organization.IsErrTeamNotExist(err) {
					ctx.APIErrorNotFound("TeamNotExist", fmt.Sprintf("Team '%s' not exist", t))
					return nil, nil
				}
				ctx.APIErrorInternal(err)
				return nil, nil
			}

			teamReviewers = append(teamReviewers, teamReviewer)
		}
	}
	return reviewers, teamReviewers
}

func apiReviewRequest(ctx *context.APIContext, opts api.PullReviewRequestOptions, isAdd bool) {
	pr, err := issues_model.GetPullRequestByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrPullRequestNotExist(err) {
			ctx.APIErrorNotFound("GetPullRequestByIndex", err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if err := pr.Issue.LoadRepo(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	permDoer, err := access_model.GetUserRepoPermission(ctx, pr.Issue.Repo, ctx.Doer)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	reviewers, teamReviewers := parseReviewersByNames(ctx, opts.Reviewers, opts.TeamReviewers)
	if ctx.Written() {
		return
	}

	var reviews []*issues_model.Review
	if isAdd {
		reviews = make([]*issues_model.Review, 0, len(reviewers))
	}

	for _, reviewer := range reviewers {
		comment, err := issue_service.ReviewRequest(ctx, pr.Issue, ctx.Doer, &permDoer, reviewer, isAdd)
		if err != nil {
			if issues_model.IsErrReviewRequestOnClosedPR(err) {
				ctx.APIError(http.StatusForbidden, err)
				return
			}
			if issues_model.IsErrNotValidReviewRequest(err) {
				ctx.APIError(http.StatusUnprocessableEntity, err)
				return
			}
			ctx.APIErrorInternal(err)
			return
		}

		if comment != nil && isAdd {
			if err = comment.LoadReview(ctx); err != nil {
				ctx.APIErrorInternal(err)
				return
			}
			reviews = append(reviews, comment.Review)
		}
	}

	if ctx.Repo.Repository.Owner.IsOrganization() && len(opts.TeamReviewers) > 0 {
		for _, teamReviewer := range teamReviewers {
			comment, err := issue_service.TeamReviewRequest(ctx, pr.Issue, ctx.Doer, teamReviewer, isAdd)
			if err != nil {
				if issues_model.IsErrReviewRequestOnClosedPR(err) {
					ctx.APIError(http.StatusForbidden, err)
					return
				}
				if issues_model.IsErrNotValidReviewRequest(err) {
					ctx.APIError(http.StatusUnprocessableEntity, err)
					return
				}
				ctx.APIErrorInternal(err)
				return
			}

			if comment != nil && isAdd {
				if err = comment.LoadReview(ctx); err != nil {
					ctx.APIErrorInternal(err)
					return
				}
				reviews = append(reviews, comment.Review)
			}
		}
	}

	if isAdd {
		apiReviews, err := convert.ToPullReviewList(ctx, reviews, ctx.Doer)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		ctx.JSON(http.StatusCreated, apiReviews)
	} else {
		ctx.Status(http.StatusNoContent)
		return
	}
}

// DismissPullReview dismiss a review for a pull request
func DismissPullReview(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/pulls/{index}/reviews/{id}/dismissals repository repoDismissPullReview
	// ---
	// summary: Dismiss a review for a pull request
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
	//     "$ref": "#/definitions/DismissPullReviewOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/PullReview"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	opts := web.GetForm(ctx).(*api.DismissPullReviewOptions)
	dismissReview(ctx, opts.Message, true, opts.Priors)
}

// UnDismissPullReview cancel to dismiss a review for a pull request
func UnDismissPullReview(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/pulls/{index}/reviews/{id}/undismissals repository repoUnDismissPullReview
	// ---
	// summary: Cancel to dismiss a review for a pull request
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
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	dismissReview(ctx, "", false, false)
}

func dismissReview(ctx *context.APIContext, msg string, isDismiss, dismissPriors bool) {
	if !ctx.Repo.IsAdmin() {
		ctx.APIError(http.StatusForbidden, "Must be repo admin")
		return
	}
	review, _, isWrong := prepareSingleReview(ctx)
	if isWrong {
		return
	}

	if review.Type != issues_model.ReviewTypeApprove && review.Type != issues_model.ReviewTypeReject {
		ctx.APIError(http.StatusForbidden, "not need to dismiss this review because it's type is not Approve or change request")
		return
	}

	_, err := pull_service.DismissReview(ctx, review.ID, ctx.Repo.Repository.ID, msg, ctx.Doer, isDismiss, dismissPriors)
	if err != nil {
		if pull_service.IsErrDismissRequestOnClosedPR(err) {
			ctx.APIError(http.StatusForbidden, err)
			return
		}
		ctx.APIErrorInternal(err)
		return
	}

	if review, err = issues_model.GetReviewByID(ctx, review.ID); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	// convert response
	apiReview, err := convert.ToPullReview(ctx, review, ctx.Doer)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, apiReview)
}
