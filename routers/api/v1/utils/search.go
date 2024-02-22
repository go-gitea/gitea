// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"net/http"

	"code.gitea.io/gitea/modules/context"
	code_indexer "code.gitea.io/gitea/modules/indexer/code"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/convert"
)

// PerformCodeSearch performs a code search on the given repos
func PerformCodeSearch(ctx *context.APIContext, repos []int64) {
	if !setting.Indexer.RepoIndexerEnabled {
		ctx.Error(http.StatusNotImplemented, "IndexerNotEnabled", "The Code Indexer is not enabled on this server")
		return
	}

	language := ctx.FormTrim("language")
	keyword := ctx.FormTrim("keyword")
	isMatch := ctx.FormOptionalBool("match").IsTrue()

	if keyword == "" {
		ctx.Error(http.StatusUnprocessableEntity, "KeywordEmpty", "The keyword can't be empty")
		return
	}

	listOptions := GetListOptions(ctx)

	if listOptions.Page <= 0 {
		listOptions.Page = 1
	}

	total, searchResults, searchResultLanguages, err := code_indexer.PerformSearch(ctx, repos,
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
