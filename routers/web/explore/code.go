// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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
	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	queryType := ctx.FormTrim("t")
	isMatch := queryType == "match"

	if keyword != "" {
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
			repoIDs, err = repo_model.FindUserCodeAccessibleRepoIDs(ctx.Doer)
			if err != nil {
				ctx.ServerError("SearchResults", err)
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
				if code_indexer.IsAvailable() {
					ctx.ServerError("SearchResults", err)
					return
				}
				ctx.Data["CodeIndexerUnavailable"] = true
			} else {
				ctx.Data["CodeIndexerUnavailable"] = !code_indexer.IsAvailable()
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
				ctx.ServerError("SearchResults", err)
				return
			}

			ctx.Data["RepoMaps"] = repoMaps
		}

		ctx.Data["Keyword"] = keyword
		ctx.Data["Language"] = language
		ctx.Data["queryType"] = queryType
		ctx.Data["SearchResults"] = searchResults
		ctx.Data["SearchResultLanguages"] = searchResultLanguages
		ctx.Data["PageIsViewCode"] = true

		pager := context.NewPagination(total, setting.UI.RepoSearchPagingNum, page, 5)
		pager.SetDefaultParams(ctx)
		pager.AddParam(ctx, "l", "Language")
		ctx.Data["Page"] = pager
	}

	ctx.HTML(http.StatusOK, tplExploreCode)
}
