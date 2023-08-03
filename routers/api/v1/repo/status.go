// Copyright 2017 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"net/http"

	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/convert"
	files_service "code.gitea.io/gitea/services/repository/files"
)

// NewCommitStatus creates a new CommitStatus
func NewCommitStatus(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/statuses/{sha} repository repoCreateStatus
	// ---
	// summary: Create a commit status
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
	// - name: sha
	//   in: path
	//   description: sha of the commit
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateStatusOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/CommitStatus"
	//   "400":
	//     "$ref": "#/responses/error"

	form := web.GetForm(ctx).(*api.CreateStatusOption)
	sha := ctx.Params("sha")
	if len(sha) == 0 {
		ctx.Error(http.StatusBadRequest, "sha not given", nil)
		return
	}
	status := &git_model.CommitStatus{
		State:       form.State,
		TargetURL:   form.TargetURL,
		Description: form.Description,
		Context:     form.Context,
	}
	if err := files_service.CreateCommitStatus(ctx, ctx.Repo.Repository, ctx.Doer, sha, status); err != nil {
		ctx.Error(http.StatusInternalServerError, "CreateCommitStatus", err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToCommitStatus(ctx, status))
}

// GetCommitStatuses returns all statuses for any given commit hash
func GetCommitStatuses(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/statuses/{sha} repository repoListStatuses
	// ---
	// summary: Get a commit's statuses
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
	// - name: sha
	//   in: path
	//   description: sha of the commit
	//   type: string
	//   required: true
	// - name: sort
	//   in: query
	//   description: type of sort
	//   type: string
	//   enum: [oldest, recentupdate, leastupdate, leastindex, highestindex]
	//   required: false
	// - name: state
	//   in: query
	//   description: type of state
	//   type: string
	//   enum: [pending, success, error, failure, warning]
	//   required: false
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
	//     "$ref": "#/responses/CommitStatusList"
	//   "400":
	//     "$ref": "#/responses/error"

	getCommitStatuses(ctx, ctx.Params("sha"))
}

// GetCommitStatusesByRef returns all statuses for any given commit ref
func GetCommitStatusesByRef(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/commits/{ref}/statuses repository repoListStatusesByRef
	// ---
	// summary: Get a commit's statuses, by branch/tag/commit reference
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
	// - name: ref
	//   in: path
	//   description: name of branch/tag/commit
	//   type: string
	//   required: true
	// - name: sort
	//   in: query
	//   description: type of sort
	//   type: string
	//   enum: [oldest, recentupdate, leastupdate, leastindex, highestindex]
	//   required: false
	// - name: state
	//   in: query
	//   description: type of state
	//   type: string
	//   enum: [pending, success, error, failure, warning]
	//   required: false
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
	//     "$ref": "#/responses/CommitStatusList"
	//   "400":
	//     "$ref": "#/responses/error"

	filter := utils.ResolveRefOrSha(ctx, ctx.Params("ref"))
	if ctx.Written() {
		return
	}

	getCommitStatuses(ctx, filter) // By default filter is maybe the raw SHA
}

func getCommitStatuses(ctx *context.APIContext, sha string) {
	if len(sha) == 0 {
		ctx.Error(http.StatusBadRequest, "ref/sha not given", nil)
		return
	}
	sha = utils.MustConvertToSHA1(ctx.Base, ctx.Repo, sha)
	repo := ctx.Repo.Repository

	listOptions := utils.GetListOptions(ctx)

	statuses, maxResults, err := git_model.GetCommitStatuses(ctx, repo, sha, &git_model.CommitStatusOptions{
		ListOptions: listOptions,
		SortType:    ctx.FormTrim("sort"),
		State:       ctx.FormTrim("state"),
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetCommitStatuses", fmt.Errorf("GetCommitStatuses[%s, %s, %d]: %w", repo.FullName(), sha, ctx.FormInt("page"), err))
		return
	}

	apiStatuses := make([]*api.CommitStatus, 0, len(statuses))
	for _, status := range statuses {
		apiStatuses = append(apiStatuses, convert.ToCommitStatus(ctx, status))
	}

	ctx.SetLinkHeader(int(maxResults), listOptions.PageSize)
	ctx.SetTotalCountHeader(maxResults)

	ctx.JSON(http.StatusOK, apiStatuses)
}

// GetCombinedCommitStatusByRef returns the combined status for any given commit hash
func GetCombinedCommitStatusByRef(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/commits/{ref}/status repository repoGetCombinedStatusByRef
	// ---
	// summary: Get a commit's combined status, by branch/tag/commit reference
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
	// - name: ref
	//   in: path
	//   description: name of branch/tag/commit
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
	//     "$ref": "#/responses/CombinedStatus"
	//   "400":
	//     "$ref": "#/responses/error"

	sha := utils.ResolveRefOrSha(ctx, ctx.Params("ref"))
	if ctx.Written() {
		return
	}

	repo := ctx.Repo.Repository

	statuses, count, err := git_model.GetLatestCommitStatus(ctx, repo.ID, sha, utils.GetListOptions(ctx))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetLatestCommitStatus", fmt.Errorf("GetLatestCommitStatus[%s, %s]: %w", repo.FullName(), sha, err))
		return
	}

	if len(statuses) == 0 {
		ctx.JSON(http.StatusOK, &api.CombinedStatus{})
		return
	}

	combiStatus := convert.ToCombinedStatus(ctx, statuses, convert.ToRepo(ctx, repo, ctx.Repo.Permission))

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, combiStatus)
}
