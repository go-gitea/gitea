// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	git_model "gitea.dev/models/git"
	pull_model "gitea.dev/models/pull"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
	"gitea.dev/services/mergequeue"
	pull_service "gitea.dev/services/pull"
)

func GetPullRequestMergeQueueEntry(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/pulls/{index}/merge-queue repository repoGetPullRequestMergeQueueEntry
	// ---
	// summary: Get the merge queue entry of a pull request
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
	//     "$ref": "#/responses/MergeQueueEntry"
	//   default:
	//     "$ref": "#/responses/error"
	pr := getPullRequestToMerge(ctx)
	if pr == nil {
		return
	}
	entry, err := pull_model.GetMergeQueueEntryByPullID(ctx, pr.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	} else if entry == nil {
		ctx.APIErrorNotFound()
		return
	}
	writeMergeQueueEntry(ctx, http.StatusOK, entry, pr.Index)
}

func AddPullRequestToMergeQueue(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/pulls/{index}/merge-queue repository repoAddPullRequestToMergeQueue
	// ---
	// summary: Add a pull request to the merge queue of its base branch
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
	//   schema:
	//     $ref: "#/definitions/AddToMergeQueueOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/MergeQueueEntry"
	//   "201":
	//     "$ref": "#/responses/MergeQueueEntry"
	//   default:
	//     "$ref": "#/responses/error"
	form := web.GetForm[*api.AddToMergeQueueOption](ctx)
	pr := getPullRequestToMerge(ctx)
	if pr == nil {
		return
	}
	if form.Jump && !ctx.Repo.Permission.IsAdmin() {
		ctx.APIError(http.StatusForbidden, "only repository admins can jump the merge queue")
		return
	}
	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, pr.BaseRepoID, pr.BaseBranch)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	} else if pb == nil || !pb.RequireMergeQueue {
		ctx.APIError(http.StatusUnprocessableEntity, "the base branch does not require a merge queue")
		return
	}
	if !checkPullMergeable(ctx, pr, pull_service.MergeCheckTypeGeneral, pb.MergeQueue.MergeMethod, false) {
		return
	}
	entry, added, err := mergequeue.Enqueue(ctx, ctx.Doer, pr, form.Jump, form.HeadCommitID)
	if err != nil {
		writeMergeError(ctx, err)
		return
	}
	writeMergeQueueEntry(ctx, util.Iif(added, http.StatusCreated, http.StatusOK), entry, pr.Index)
}

func writeMergeQueueEntry(ctx *context.APIContext, status int, entry *pull_model.MergeQueueEntry, prIndex int64) {
	position, err := pull_model.GetMergeQueuePosition(ctx, entry)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	_, enqueuer, err := user_model.GetPossibleUserByID(ctx, entry.EnqueuerID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(status, convert.ToMergeQueueEntry(ctx, entry, position, prIndex, enqueuer))
}

func RemovePullRequestFromMergeQueue(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/pulls/{index}/merge-queue repository repoRemovePullRequestFromMergeQueue
	// ---
	// summary: Remove a pull request from the merge queue
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
	//   "204":
	//     "$ref": "#/responses/empty"
	//   default:
	//     "$ref": "#/responses/error"
	pr := getPullRequestToMerge(ctx)
	if pr == nil {
		return
	}
	if err := mergequeue.Dequeue(ctx, ctx.Doer, ctx.Repo.Permission, pr); err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func ListMergeQueue(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/merge-queue/{branch} repository repoListMergeQueue
	// ---
	// summary: List the merge queue of a branch in merge order
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
	// - name: branch
	//   in: path
	//   description: base branch of the queue
	//   type: string
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
	//     "$ref": "#/responses/MergeQueueEntryList"
	//   default:
	//     "$ref": "#/responses/error"
	branch := ctx.PathParam("*")
	if rule, err := mergequeue.GetEnabledRule(ctx, ctx.Repo.Repository, branch); err != nil {
		ctx.APIErrorInternal(err)
		return
	} else if rule == nil {
		ctx.APIErrorNotFound()
		return
	}
	queue, err := mergequeue.GetQueue(ctx, ctx.Repo.Repository, branch)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	listOptions := utils.GetListOptions(ctx)
	page := util.PaginateSlice(queue, listOptions.Page, listOptions.PageSize)

	result := make([]*api.MergeQueueEntry, len(page))
	for i, entry := range page {
		result[i] = convert.ToMergeQueueEntry(ctx, entry.MergeQueueEntry, entry.Position, entry.PullRequest.Index, entry.Enqueuer)
	}
	ctx.SetLinkHeader(int64(len(queue)), listOptions.PageSize)
	ctx.SetTotalCountHeader(int64(len(queue)))
	ctx.JSON(http.StatusOK, result)
}

func ClearMergeQueue(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/merge-queue/{branch} repository repoClearMergeQueue
	// ---
	// summary: Remove all pull requests from the merge queue of a branch
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
	// - name: branch
	//   in: path
	//   description: base branch of the queue
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   default:
	//     "$ref": "#/responses/error"
	if err := mergequeue.Clear(ctx, ctx.Doer, ctx.Repo.Repository, ctx.PathParam("*")); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
