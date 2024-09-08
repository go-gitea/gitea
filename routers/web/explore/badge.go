// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package explore

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
)

func RenderBadgeSearch(ctx *context.Context, opts *user_model.SearchBadgeOptions, tplName base.TplName) {
	// Sitemap index for sitemap paths
	opts.Page = int(ctx.PathParamInt64("idx"))
	if opts.Page <= 1 {
		opts.Page = ctx.FormInt("page")
	}
	if opts.Page <= 1 {
		opts.Page = 1
	}

	var (
		badges  []*user_model.Badge
		count   int64
		err     error
		orderBy db.SearchOrderBy
	)

	// we can not set orderBy to `models.SearchOrderByXxx`, because there may be a JOIN in the statement, different tables may have the same name columns

	sortOrder := ctx.FormString("sort")
	if sortOrder == "" {
		sortOrder = setting.UI.ExploreDefaultSort
	}
	ctx.Data["SortType"] = sortOrder

	switch sortOrder {
	case "newest":
		orderBy = "`badge`.id DESC"
	case "oldest":
		orderBy = "`badge`.id ASC"
	case "reversealphabetically":
		orderBy = "`badge`.slug DESC"
	case "alphabetically":
		orderBy = "`badge`.slug ASC"
	default:
		// in case the sortType is not valid, we set it to recent update
		ctx.Data["SortType"] = "oldest"
		orderBy = "`badge`.id ASC"
	}

	opts.Keyword = ctx.FormTrim("q")
	opts.OrderBy = orderBy
	if len(opts.Keyword) == 0 || isKeywordValid(opts.Keyword) {
		badges, count, err = user_model.SearchBadges(ctx, opts)
		if err != nil {
			ctx.ServerError("SearchBadges", err)
			return
		}
	}

	ctx.Data["Keyword"] = opts.Keyword
	ctx.Data["Total"] = count
	ctx.Data["Badges"] = badges

	pager := context.NewPagination(int(count), opts.PageSize, opts.Page, 5)
	pager.SetDefaultParams(ctx)
	for paramKey, paramVal := range opts.ExtraParamStrings {
		pager.AddParamString(paramKey, paramVal)
	}
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplName)
}
