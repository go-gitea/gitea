// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	code_indexer "code.gitea.io/gitea/modules/indexer/code"
	"code.gitea.io/gitea/modules/setting"
)

const (
	tplSearch       base.TplName = "repo/search"
	tplSearchResult base.TplName = "repo/search_result"
)

// Search render repository search page
func Search(ctx *context.Context) {
	if !setting.Indexer.RepoIndexerEnabled {
		ctx.Redirect(ctx.Repo.RepoLink)
		return
	}

	language := ctx.FormTrim("l")
	keyword := ctx.FormTrim("q")

	queryType := ctx.FormTrim("t")
	isMatch := queryType == "match"

	ctx.Data["Keyword"] = keyword
	ctx.Data["Language"] = language
	ctx.Data["queryType"] = queryType
	ctx.Data["PageIsViewCode"] = true

	isHtmxRequest := len(ctx.Req.Header.Values("HX-Request")) > 0

	template := tplSearch
	if isHtmxRequest {
		template = tplSearchResult
	}

	if keyword == "" {
		ctx.HTML(http.StatusOK, template)
		return
	}

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	total, searchResults, searchResultLanguages, err := code_indexer.PerformSearch(ctx, []int64{ctx.Repo.Repository.ID},
		language, keyword, page, setting.UI.RepoSearchPagingNum, isMatch)
	if err != nil {
		if code_indexer.IsAvailable(ctx) {
			ctx.ServerError("SearchResults", err)
			return
		}
		ctx.Data["CodeIndexerUnavailable"] = true
	} else {
		ctx.Data["CodeIndexerUnavailable"] = !code_indexer.IsAvailable(ctx)
	}

	ctx.Data["SourcePath"] = ctx.Repo.Repository.Link()
	ctx.Data["SearchResults"] = searchResults
	ctx.Data["SearchResultLanguages"] = searchResultLanguages

	pager := context.NewPagination(total, setting.UI.RepoSearchPagingNum, page, 5)
	pager.SetDefaultParams(ctx)
	pager.AddParam(ctx, "l", "Language")
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, template)
}
