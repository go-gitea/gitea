// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"cmp"
	"errors"
	"net/http"

	repo_model "gitea.dev/models/repo"
	api "gitea.dev/modules/structs"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
	repo_service "gitea.dev/services/repository"
)

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
	// - name: per_page
	//   in: query
	//   description: page size of results, alias of limit
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/ContributorList"
	//   "202":
	//     description: The contributors are still being counted, retry later
	//   "204":
	//     description: The repository is empty
	//   "404":
	//     "$ref": "#/responses/notFound"

	if ctx.Repo.Repository.IsEmpty {
		ctx.Status(http.StatusNoContent)
		return
	}
	if err := repo_service.EnsureContributorStats(ctx, ctx.Repo.Repository); errors.Is(err, repo_service.ErrAwaitGeneration) {
		ctx.Status(http.StatusAccepted)
		return
	} else if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	listOptions := utils.GetListOptions(ctx)
	listOptions.PageSize = convert.ToCorrectPageSize(cmp.Or(ctx.FormInt("per_page"), ctx.FormInt("limit")))
	contributors, total, err := repo_service.FindRepoContributors(ctx, ctx.Repo.Repository, repo_model.FindRepoContributorsOptions{
		ListOptions: listOptions,
		OnlyUsers:   !ctx.FormBool("anon"),
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiContributors := make([]*api.Contributor, len(contributors))
	for i, contributor := range contributors {
		apiContributor := &api.Contributor{Type: "Anonymous", Name: contributor.Name, Email: contributor.Email, Contributions: contributor.Commits, Additions: contributor.Additions, Deletions: contributor.Deletions}
		if user := contributor.User; user != nil {
			apiContributor.Type, apiContributor.ID, apiContributor.Login = string(convert.UserTypeToString(user.Type)), user.ID, user.Name
			apiContributor.AvatarURL, apiContributor.HTMLURL = user.AvatarLink(ctx), user.HTMLURL(ctx)
		}
		apiContributors[i] = apiContributor
	}
	ctx.SetLinkHeader(total, listOptions.PageSize)
	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, apiContributors)
}
