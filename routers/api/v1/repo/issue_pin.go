// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

// PinIssue pins a issue
func PinIssue(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/issues/{index}/pin issue pinIssue
	// ---
	// summary: Pin an Issue
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
	//   description: index of issue to pin
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
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.APIErrorNotFound()
		} else if issues_model.IsErrIssueMaxPinReached(err) {
			ctx.APIError(http.StatusBadRequest, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	// If we don't do this, it will crash when trying to add the pin event to the comment history
	err = issue.LoadRepo(ctx)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	err = issues_model.PinIssue(ctx, issue, ctx.Doer)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// UnpinIssue unpins a Issue
func UnpinIssue(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issues/{index}/pin issue unpinIssue
	// ---
	// summary: Unpin an Issue
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
	//   description: index of issue to unpin
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
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	// If we don't do this, it will crash when trying to add the unpin event to the comment history
	err = issue.LoadRepo(ctx)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	err = issues_model.UnpinIssue(ctx, issue, ctx.Doer)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// MoveIssuePin moves a pinned Issue to a new Position
func MoveIssuePin(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/issues/{index}/pin/{position} issue moveIssuePin
	// ---
	// summary: Moves the Pin to the given Position
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
	//   description: index of issue
	//   type: integer
	//   format: int64
	//   required: true
	// - name: position
	//   in: path
	//   description: the new position
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
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	err = issues_model.MovePin(ctx, issue, int(ctx.PathParamInt64("position")))
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// ListPinnedIssues returns a list of all pinned Issues
func ListPinnedIssues(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/pinned repository repoListPinnedIssues
	// ---
	// summary: List a repo's pinned issues
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
	// responses:
	//   "200":
	//     "$ref": "#/responses/IssueList"
	//   "404":
	//     "$ref": "#/responses/notFound"
	issues, err := issues_model.GetPinnedIssues(ctx, ctx.Repo.Repository.ID, false)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToAPIIssueList(ctx, ctx.Doer, issues))
}

// ListPinnedPullRequests returns a list of all pinned PRs
func ListPinnedPullRequests(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/pulls/pinned repository repoListPinnedPullRequests
	// ---
	// summary: List a repo's pinned pull requests
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
	// responses:
	//   "200":
	//     "$ref": "#/responses/PullRequestList"
	//   "404":
	//     "$ref": "#/responses/notFound"
	issues, err := issues_model.GetPinnedIssues(ctx, ctx.Repo.Repository.ID, true)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiPrs := make([]*api.PullRequest, len(issues))
	if err := issues.LoadPullRequests(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	for i, currentIssue := range issues {
		pr := currentIssue.PullRequest
		if err = pr.LoadAttributes(ctx); err != nil {
			ctx.APIErrorInternal(err)
			return
		}

		if err = pr.LoadBaseRepo(ctx); err != nil {
			ctx.APIErrorInternal(err)
			return
		}

		if err = pr.LoadHeadRepo(ctx); err != nil {
			ctx.APIErrorInternal(err)
			return
		}

		apiPrs[i] = convert.ToAPIPullRequest(ctx, pr, ctx.Doer)
	}

	ctx.JSON(http.StatusOK, &apiPrs)
}

// AreNewIssuePinsAllowed returns if new issues pins are allowed
func AreNewIssuePinsAllowed(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/new_pin_allowed repository repoNewPinAllowed
	// ---
	// summary: Returns if new Issue Pins are allowed
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
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepoNewIssuePinsAllowed"
	//   "404":
	//     "$ref": "#/responses/notFound"
	pinsAllowed := api.NewIssuePinsAllowed{}
	var err error

	pinsAllowed.Issues, err = issues_model.IsNewPinAllowed(ctx, ctx.Repo.Repository.ID, false)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	pinsAllowed.PullRequests, err = issues_model.IsNewPinAllowed(ctx, ctx.Repo.Repository.ID, true)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, pinsAllowed)
}
