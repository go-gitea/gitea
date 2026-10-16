// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	repo_model "gitea.dev/models/repo"
	api "gitea.dev/modules/structs"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
	repo_service "gitea.dev/services/repository"
)

// ListContributors lists the commit authors of the default branch of a repository
func ListContributors(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/contributors repository repoListContributors
	// ---
	// summary: List the commit authors of the default branch of a repository, most commits first
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
	// - name: anon
	//   in: query
	//   description: include authors that are not matched to a user
	//   type: boolean
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
	//     "$ref": "#/responses/ContributorList"
	//   "202":
	//     description: The contributors are still being counted, retry later
	//   "404":
	//     "$ref": "#/responses/notFound"

	if err := repo_service.EnsureContributorStats(ctx, ctx.Repo.Repository); errors.Is(err, repo_service.ErrAwaitGeneration) {
		ctx.Status(http.StatusAccepted)
		return
	} else if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	contributors, total, err := repo_service.FindRepoContributors(ctx, ctx.Repo.Repository, repo_model.FindRepoContributorsOptions{
		ListOptions: utils.GetListOptions(ctx),
		OnlyUsers:   !ctx.FormBool("anon"),
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiContributors := make([]*api.Contributor, len(contributors))
	for i, c := range contributors {
		apiContributors[i] = convert.ToContributor(ctx, c)
	}
	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, apiContributors)
}
