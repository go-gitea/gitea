// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"path"
	"strings"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/search"
	"code.gitea.io/gitea/modules/setting"

	"github.com/Unknwon/paginater"
)

const tplSearch base.TplName = "repo/search"

// Search render repository search page
func Search(ctx *context.Context) {
	if !setting.Indexer.RepoIndexerEnabled {
		ctx.Redirect(ctx.Repo.RepoLink, 302)
		return
	}
	keyword := strings.TrimSpace(ctx.Query("q"))
	page := ctx.QueryInt("page")
	if page <= 0 {
		page = 1
	}
	total, searchResults, err := search.PerformSearch([]int64{ctx.Repo.Repository.ID},
		keyword, page, setting.UI.RepoSearchPagingNum)
	if err != nil {
		ctx.ServerError("SearchResults", err)
		return
	}
	ctx.Data["Keyword"] = keyword
	pager := paginater.New(total, setting.UI.RepoSearchPagingNum, page, 5)
	ctx.Data["Page"] = pager
	ctx.Data["SourcePath"] = setting.AppSubURL + "/" +
		path.Join(ctx.Repo.Repository.Owner.Name, ctx.Repo.Repository.Name, "src", "branch", ctx.Repo.Repository.DefaultBranch)
	ctx.Data["SearchResults"] = searchResults
	ctx.Data["RequireHighlightJS"] = true
	ctx.Data["PageIsViewCode"] = true
	ctx.HTML(200, tplSearch)
}
