// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/modules/context"
	code_indexer "code.gitea.io/gitea/modules/indexer/code"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/convert"
)

// CodeSearch Performs a code search on a Repo
func CodeSearch(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/code_search repository repoCodeSearch
	// ---
	// summary: Performs a code search on a Repo
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
	// - name: keyword
	//   in: query
	//   description: the keyword the search for
	//   type: string
	// - name: language
	//   in: query
	//   description: filter results by language
	//   type: string
	// - name: match
	//   in: query
	//   description: only exact match (defaults to true)
	//   type: boolean
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepoCodeSearch"
	if !setting.Indexer.RepoIndexerEnabled {
		ctx.Error(http.StatusInternalServerError, "IndexerNotEnabled", "The Code Indexer is not enabled on this server")
		return
	}

	language := ctx.FormTrim("language")
	keyword := ctx.FormTrim("keyword")
	isMatch := !ctx.FormOptionalBool("match").IsFalse()

	if keyword == "" {
		ctx.Error(http.StatusUnprocessableEntity, "KeywordEmpty", "The keyword can't be empty")
		return
	}

	listOptions := utils.GetListOptions(ctx)

	if listOptions.Page <= 0 {
		listOptions.Page = 1
	}

	total, searchResults, searchResultLanguages, err := code_indexer.PerformSearch(ctx, []int64{ctx.Repo.Repository.ID},
		language, keyword, listOptions.Page, listOptions.PageSize, isMatch)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	response := api.RepoCodeSearchAPIResponse{
		Total:                 total,
		SearchResults:         convert.ToIndexerSearchResultList(searchResults),
		SearchResultLanguages: convert.ToIndexerSearchResultLanguagesList(searchResultLanguages),
	}

	pager := context.NewPagination(total, listOptions.PageSize, listOptions.Page, 5)

	ctx.SetLinkHeader(pager.Paginater.TotalPages(), listOptions.PageSize)
	ctx.SetTotalCountHeader(int64(pager.Paginater.TotalPages()))
	ctx.JSON(http.StatusOK, response)
}
