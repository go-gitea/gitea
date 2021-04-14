// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"path"
	"strings"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	code_indexer "code.gitea.io/gitea/modules/indexer/code"
	"code.gitea.io/gitea/modules/setting"
)

const tplSearch base.TplName = "repo/search"

// Search render repository search page
func Search(ctx *context.Context) {
	if !setting.Indexer.RepoIndexerEnabled {
		ctx.Redirect(ctx.Repo.RepoLink, 302)
		return
	}
	language := strings.TrimSpace(ctx.Query("l"))
	keyword := strings.TrimSpace(ctx.Query("q"))
	page := ctx.QueryInt("page")
	if page <= 0 {
		page = 1
	}
	queryType := strings.TrimSpace(ctx.Query("t"))
	isMatch := queryType == "match"

	total, searchResults, searchResultLanguages, err := code_indexer.PerformSearch([]int64{ctx.Repo.Repository.ID},
		language, keyword, page, setting.UI.RepoSearchPagingNum, isMatch)
	if err != nil {
		ctx.ServerError("SearchResults", err)
		return
	}
	ctx.Data["Keyword"] = keyword
	ctx.Data["Language"] = language
	ctx.Data["queryType"] = queryType
	ctx.Data["SourcePath"] = path.Join(setting.AppSubURL, ctx.Repo.Repository.Owner.Name, ctx.Repo.Repository.Name)
	ctx.Data["SearchResults"] = searchResults
	ctx.Data["SearchResultLanguages"] = searchResultLanguages
	ctx.Data["RequireHighlightJS"] = true
	ctx.Data["PageIsViewCode"] = true

	pager := context.NewPagination(total, setting.UI.RepoSearchPagingNum, page, 5)
	pager.SetDefaultParams(ctx)
	pager.AddParam(ctx, "l", "Language")
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplSearch)
}
