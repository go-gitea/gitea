// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/base"
	code_indexer "code.gitea.io/gitea/modules/indexer/code"
	"code.gitea.io/gitea/modules/setting"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/context"
)

const (
	tplUserCode base.TplName = "user/code"
)

// CodeSearch render user/organization code search page
func CodeSearch(ctx *context.Context) {
	if !setting.Indexer.RepoIndexerEnabled {
		ctx.Redirect(ctx.ContextUser.HomeLink())
		return
	}
	shared_user.PrepareContextForProfileBigAvatar(ctx)
	shared_user.RenderUserHeader(ctx)

	if err := shared_user.LoadHeaderCount(ctx); err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

	ctx.Data["IsPackageEnabled"] = setting.Packages.Enabled
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled
	ctx.Data["Title"] = ctx.Tr("explore.code")

	language := ctx.FormTrim("l")
	keyword := ctx.FormTrim("q")

	queryType := ctx.FormTrim("t")
	isFuzzy := queryType != "match"
	wikis := ctx.FormOptionalBool("wikis")

	ctx.Data["Keyword"] = keyword
	ctx.Data["Language"] = language
	ctx.Data["queryType"] = queryType
	ctx.Data["IsCodePage"] = true
	ctx.Data["Wikis"] = wikis

	if keyword == "" {
		ctx.HTML(http.StatusOK, tplUserCode)
		return
	}

	var (
		repoIDs []int64
		err     error
	)

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	repoIDs, err = repo_model.FindUserCodeAccessibleOwnerRepoIDs(ctx, ctx.ContextUser.ID, ctx.Doer)
	if err != nil {
		ctx.ServerError("FindUserCodeAccessibleOwnerRepoIDs", err)
		return
	}

	var (
		total                 int
		searchResults         []*code_indexer.Result
		searchResultLanguages []*code_indexer.SearchResultLanguages
	)

	if len(repoIDs) > 0 {
		total, searchResults, searchResultLanguages, err = code_indexer.PerformSearch(ctx, &code_indexer.SearchOptions{
			RepoIDs:        repoIDs,
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

		repoMaps, err := repo_model.GetRepositoriesMapByIDs(ctx, loadRepoIDs)
		if err != nil {
			ctx.ServerError("GetRepositoriesMapByIDs", err)
			return
		}

		ctx.Data["RepoMaps"] = repoMaps
	}
	ctx.Data["SearchResults"] = searchResults
	ctx.Data["SearchResultLanguages"] = searchResultLanguages

	pager := context.NewPagination(total, setting.UI.RepoSearchPagingNum, page, 5)
	pager.SetDefaultParams(ctx)
	pager.AddParam(ctx, "l", "Language")
	if wikis.Has() {
		pager.AddParamString("wikis", fmt.Sprintf("%v", wikis.Value()))
	}
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplUserCode)
}
