// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package explore

import (
	"net/http"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	code_indexer "code.gitea.io/gitea/modules/indexer/code"
	"code.gitea.io/gitea/modules/setting"
)

const (
	// tplExploreCode explore code page template
	tplExploreCode base.TplName = "explore/code"
)

// Code render explore code page
func Code(ctx *context.Context) {
	if !setting.Indexer.RepoIndexerEnabled {
		ctx.Redirect(setting.AppSubURL + "/explore")
		return
	}

	ctx.Data["UsersIsDisabled"] = setting.Service.Explore.DisableUsersPage
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled
	ctx.Data["Title"] = ctx.Tr("explore")
	ctx.Data["PageIsExplore"] = true
	ctx.Data["PageIsExploreCode"] = true

	language := ctx.FormTrim("l")
	keyword := ctx.FormTrim("q")

	queryType := ctx.FormTrim("t")
	isMatch := queryType == "match"

	ctx.Data["Keyword"] = keyword
	ctx.Data["Language"] = language
	ctx.Data["queryType"] = queryType
	ctx.Data["PageIsViewCode"] = true

	if keyword == "" {
		ctx.HTML(http.StatusOK, tplExploreCode)
		return
	}

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	var (
		repoIDs []int64
		err     error
		isAdmin bool
	)
	if ctx.Doer != nil {
		isAdmin = ctx.Doer.IsAdmin
	}

	// guest user or non-admin user
	if ctx.Doer == nil || !isAdmin {
		repoIDs, err = repo_model.FindUserCodeAccessibleRepoIDs(ctx, ctx.Doer)
		if err != nil {
			ctx.ServerError("FindUserCodeAccessibleRepoIDs", err)
			return
		}
	}

	var (
		total                 int
		searchResults         []*code_indexer.Result
		searchResultLanguages []*code_indexer.SearchResultLanguages
	)

	if (len(repoIDs) > 0) || isAdmin {
		total, searchResults, searchResultLanguages, err = code_indexer.PerformSearch(ctx, repoIDs, language, keyword, page, setting.UI.RepoSearchPagingNum, isMatch)
		if err != nil {
			if code_indexer.IsAvailable(ctx) {
				ctx.ServerError("SearchResults", err)
				return
			}
			ctx.Data["CodeIndexerUnavailable"] = true
		} else {
			ctx.Data["CodeIndexerUnavailable"] = !code_indexer.IsAvailable(ctx)
		}

		loadRepoIDs := make([]int64, 0, len(searchResults))
		for _, result := range searchResults {
			var find bool
			for _, id := range loadRepoIDs {
				if id == result.RepoID {
					find = true
					break
				}
			}
			if !find {
				loadRepoIDs = append(loadRepoIDs, result.RepoID)
			}
		}

		repoMaps, err := repo_model.GetRepositoriesMapByIDs(loadRepoIDs)
		if err != nil {
			ctx.ServerError("GetRepositoriesMapByIDs", err)
			return
		}

		ctx.Data["RepoMaps"] = repoMaps

		if len(loadRepoIDs) != len(repoMaps) {
			// Remove deleted repos from search results
			cleanedSearchResults := make([]*code_indexer.Result, 0, len(repoMaps))
			for _, sr := range searchResults {
				if _, found := repoMaps[sr.RepoID]; found {
					cleanedSearchResults = append(cleanedSearchResults, sr)
				}
			}

			searchResults = cleanedSearchResults
		}
	}

	ctx.Data["SearchResults"] = searchResults
	ctx.Data["SearchResultLanguages"] = searchResultLanguages

	pager := context.NewPagination(total, setting.UI.RepoSearchPagingNum, page, 5)
	pager.SetDefaultParams(ctx)
	pager.AddParam(ctx, "l", "Language")
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplExploreCode)
}
