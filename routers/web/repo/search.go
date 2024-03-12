// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/base"
	code_indexer "code.gitea.io/gitea/modules/indexer/code"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
)

const tplSearch base.TplName = "repo/search"

// Search render repository search page
func Search(ctx *context.Context) {
	if !setting.Indexer.RepoIndexerEnabled {
		ctx.Redirect(ctx.Repo.RepoLink)
		return
	}

	language := ctx.FormTrim("l")
	keyword := ctx.FormTrim("q")

	queryType := ctx.FormTrim("t")
	isFuzzy := queryType != "match"
	wikis := ctx.FormOptionalBool("wikis")

	ctx.Data["Keyword"] = keyword
	ctx.Data["Language"] = language
	ctx.Data["queryType"] = queryType
	ctx.Data["PageIsViewCode"] = true
	ctx.Data["Wikis"] = wikis

	if keyword == "" {
		ctx.HTML(http.StatusOK, tplSearch)
		return
	}

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	total, searchResults, searchResultLanguages, err := code_indexer.PerformSearch(ctx, &code_indexer.SearchOptions{
		RepoIDs:        []int64{ctx.Repo.Repository.ID},
		Keyword:        keyword,
		IsKeywordFuzzy: isFuzzy,
		IsWiki:         wikis,
		Language:       language,
		Paginator: &db.ListOptions{
			Page:     page,
			PageSize: setting.UI.RepoSearchPagingNum,
		},
	})
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
	if wikis.Has() {
		pager.AddParamString("wikis", fmt.Sprintf("%v", wikis.Value()))
	}
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplSearch)
}
