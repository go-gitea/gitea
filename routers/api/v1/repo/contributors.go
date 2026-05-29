// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	contribution_model "gitea.dev/models/repo/contribution"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	api "gitea.dev/modules/structs"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/context"
)

// ListContributors lists repository contributors.
func ListContributors(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/contributors repository repoListContributors
	// ---
	// summary: List repository contributors
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
	//   description: include anonymous contributors
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
	//   "404":
	//     "$ref": "#/responses/notFound"

	includeAnonymous := ctx.FormBool("anon")
	contributors, total, err := contribution_model.GetRepoContributors(ctx, ctx.Repo.Repository.ID, includeAnonymous, utils.GetListOptions(ctx))
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	userIDs := container.Set[int64]{}
	for _, contributor := range contributors {
		if contributor.UserID > 0 {
			userIDs.Add(contributor.UserID)
		}
	}
	usersMap, err := user_model.GetUsersMapByIDs(ctx, userIDs.Values())
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	result := make([]*api.Contributor, 0, len(contributors))
	for _, contributor := range contributors {
		c := api.Contributor{
			Name:          contributor.AuthorName,
			Email:         contributor.Email,
			Contributions: contributor.Commits,
			Additions:     contributor.Additions,
			Deletions:     contributor.Deletions,
			Commits:       contributor.Commits,
			FilesChanged:  contributor.ChangedFiles,
		}
		if user := usersMap[contributor.UserID]; user != nil {
			c.Login = user.Name
			c.ID = user.ID
			c.AvatarURL = user.AvatarLink(ctx)
			c.HTMLURL = user.HTMLURL(ctx)
			c.Email = user.GetPlaceholderEmail()
		}
		result = append(result, &c)
	}

	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, result)
}
