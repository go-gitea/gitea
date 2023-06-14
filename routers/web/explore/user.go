// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package explore

import (
	"bytes"
	"net/http"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/sitemap"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
)

const (
	// tplExploreUsers explore users page template
	tplExploreUsers base.TplName = "explore/users"
)

// UserSearchDefaultSortType is the default sort type for user search
const (
	UserSearchDefaultSortType  = "recentupdate"
	UserSearchDefaultAdminSort = "alphabetically"
)

var nullByte = []byte{0x00}

func isKeywordValid(keyword string) bool {
	return !bytes.Contains([]byte(keyword), nullByte)
}

// RenderUserSearch render user search page
func RenderUserSearch(ctx *context.Context, opts *user_model.SearchUserOptions, tplName base.TplName) {
	// Sitemap index for sitemap paths
	opts.Page = int(ctx.ParamsInt64("idx"))
	isSitemap := ctx.Params("idx") != ""
	if opts.Page <= 1 {
		opts.Page = ctx.FormInt("page")
	}
	if opts.Page <= 1 {
		opts.Page = 1
	}

	if isSitemap {
		opts.PageSize = setting.UI.SitemapPagingNum
	}

	var (
		users   []*user_model.User
		count   int64
		err     error
		orderBy db.SearchOrderBy
	)

	// we can not set orderBy to `models.SearchOrderByXxx`, because there may be a JOIN in the statement, different tables may have the same name columns

	ctx.Data["SortType"] = ctx.FormString("sort")
	switch ctx.FormString("sort") {
	case "newest":
		orderBy = "`user`.id DESC"
	case "oldest":
		orderBy = "`user`.id ASC"
	case "leastupdate":
		orderBy = "`user`.updated_unix ASC"
	case "reversealphabetically":
		orderBy = "`user`.name DESC"
	case "lastlogin":
		orderBy = "`user`.last_login_unix ASC"
	case "reverselastlogin":
		orderBy = "`user`.last_login_unix DESC"
	case "alphabetically":
		orderBy = "`user`.name ASC"
	case "recentupdate":
		fallthrough
	default:
		// in case the sortType is not valid, we set it to recentupdate
		ctx.Data["SortType"] = "recentupdate"
		orderBy = "`user`.updated_unix DESC"
	}

	opts.Keyword = ctx.FormTrim("q")
	opts.OrderBy = orderBy
	if len(opts.Keyword) == 0 || isKeywordValid(opts.Keyword) {
		users, count, err = user_model.SearchUsers(opts)
		if err != nil {
			ctx.ServerError("SearchUsers", err)
			return
		}
	}
	if isSitemap {
		m := sitemap.NewSitemap()
		for _, item := range users {
			m.Add(sitemap.URL{URL: item.HTMLURL(), LastMod: item.UpdatedUnix.AsTimePtr()})
		}
		ctx.Resp.Header().Set("Content-Type", "text/xml")
		if _, err := m.WriteTo(ctx.Resp); err != nil {
			log.Error("Failed writing sitemap: %v", err)
		}
		return
	}

	ctx.Data["Keyword"] = opts.Keyword
	ctx.Data["Total"] = count
	ctx.Data["Users"] = users
	ctx.Data["UsersTwoFaStatus"] = user_model.UserList(users).GetTwoFaStatus()
	ctx.Data["ShowUserEmail"] = setting.UI.ShowUserEmail
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled

	pager := context.NewPagination(int(count), opts.PageSize, opts.Page, 5)
	pager.SetDefaultParams(ctx)
	for paramKey, paramVal := range opts.ExtraParamStrings {
		pager.AddParamString(paramKey, paramVal)
	}
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplName)
}

// Users render explore users page
func Users(ctx *context.Context) {
	if setting.Service.Explore.DisableUsersPage {
		ctx.Redirect(setting.AppSubURL + "/explore/repos")
		return
	}
	ctx.Data["Title"] = ctx.Tr("explore")
	ctx.Data["PageIsExplore"] = true
	ctx.Data["PageIsExploreUsers"] = true
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled

	if ctx.FormString("sort") == "" {
		ctx.SetFormString("sort", UserSearchDefaultSortType)
	}

	RenderUserSearch(ctx, &user_model.SearchUserOptions{
		Actor:       ctx.Doer,
		Type:        user_model.UserTypeIndividual,
		ListOptions: db.ListOptions{PageSize: setting.UI.ExplorePagingNum},
		IsActive:    util.OptionalBoolTrue,
		Visible:     []structs.VisibleType{structs.VisibleTypePublic, structs.VisibleTypeLimited, structs.VisibleTypePrivate},
	}, tplExploreUsers)
}
