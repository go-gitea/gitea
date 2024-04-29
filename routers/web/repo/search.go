// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	code_indexer "code.gitea.io/gitea/modules/indexer/code"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
)

const tplSearch base.TplName = "repo/search"

// Search render repository search page
func Search(ctx *context.Context) {
	language := ctx.FormTrim("l")
	keyword := ctx.FormTrim("q")

	isFuzzy := ctx.FormOptionalBool("fuzzy").ValueOrDefault(true)

	ctx.Data["Keyword"] = keyword
	ctx.Data["Language"] = language
	ctx.Data["IsFuzzy"] = isFuzzy
	ctx.Data["PageIsViewCode"] = true

	if keyword == "" {
		ctx.HTML(http.StatusOK, tplSearch)
		return
	}

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	var total int
	var searchResults []*code_indexer.Result
	var searchResultLanguages []*code_indexer.SearchResultLanguages
	if setting.Indexer.RepoIndexerEnabled {
		var err error
		total, searchResults, searchResultLanguages, err = code_indexer.PerformSearch(ctx, &code_indexer.SearchOptions{
			RepoIDs:        []int64{ctx.Repo.Repository.ID},
			Keyword:        keyword,
			IsKeywordFuzzy: isFuzzy,
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
	} else {
		res, err := git.GrepSearch(ctx, ctx.Repo.GitRepo, keyword, git.GrepOptions{ContextLineNumber: 3, IsFuzzy: isFuzzy})
		if err != nil {
			ctx.ServerError("GrepSearch", err)
			return
		}
		total = len(res)
		pageStart := min((page-1)*setting.UI.RepoSearchPagingNum, len(res))
		pageEnd := min(page*setting.UI.RepoSearchPagingNum, len(res))
		res = res[pageStart:pageEnd]
		for _, r := range res {
			searchResults = append(searchResults, &code_indexer.Result{
				RepoID:   ctx.Repo.Repository.ID,
				Filename: r.Filename,
				CommitID: ctx.Repo.CommitID,
				// UpdatedUnix: not supported yet
				// Language:    not supported yet
				// Color:       not supported yet
				Lines: code_indexer.HighlightSearchResultCode(r.Filename, "", r.LineNumbers, strings.Join(r.LineCodes, "\n")),
			})
		}
	}

	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled
	ctx.Data["Repo"] = ctx.Repo.Repository
	ctx.Data["SearchResults"] = searchResults
	ctx.Data["SearchResultLanguages"] = searchResultLanguages

	pager := context.NewPagination(total, setting.UI.RepoSearchPagingNum, page, 5)
	pager.SetDefaultParams(ctx)
	pager.AddParamString("l", language)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplSearch)
}
