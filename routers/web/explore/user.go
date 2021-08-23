// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package explore

import (
	"bytes"
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
)

const (
	// tplExploreUsers explore users page template
	tplExploreUsers base.TplName = "explore/users"
)

var (
	nullByte = []byte{0x00}
)

func isKeywordValid(keyword string) bool {
	return !bytes.Contains([]byte(keyword), nullByte)
}

// RenderUserSearch render user search page
func RenderUserSearch(ctx *context.Context, opts *models.SearchUserOptions, tplName base.TplName) {
	opts.Page = ctx.FormInt("page")
	if opts.Page <= 1 {
		opts.Page = 1
	}

	var (
		users   []*models.User
		count   int64
		err     error
		orderBy models.SearchOrderBy
	)

	ctx.Data["SortType"] = ctx.FormString("sort")
	switch ctx.FormString("sort") {
	case "newest":
		orderBy = models.SearchOrderByIDReverse
	case "oldest":
		orderBy = models.SearchOrderByID
	case "recentupdate":
		orderBy = models.SearchOrderByRecentUpdated
	case "leastupdate":
		orderBy = models.SearchOrderByLeastUpdated
	case "reversealphabetically":
		orderBy = models.SearchOrderByAlphabeticallyReverse
	case "alphabetically":
		orderBy = models.SearchOrderByAlphabetically
	default:
		ctx.Data["SortType"] = "alphabetically"
		orderBy = models.SearchOrderByAlphabetically
	}

	opts.Keyword = ctx.FormTrim("q")
	opts.OrderBy = orderBy
	if len(opts.Keyword) == 0 || isKeywordValid(opts.Keyword) {
		users, count, err = models.SearchUsers(opts)
		if err != nil {
			ctx.ServerError("SearchUsers", err)
			return
		}
	}
	ctx.Data["Keyword"] = opts.Keyword
	ctx.Data["Total"] = count
	ctx.Data["Users"] = users
	ctx.Data["UsersTwoFaStatus"] = models.UserList(users).GetTwoFaStatus()
	ctx.Data["ShowUserEmail"] = setting.UI.ShowUserEmail
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled

	pager := context.NewPagination(int(count), opts.PageSize, opts.Page, 5)
	pager.SetDefaultParams(ctx)
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

	RenderUserSearch(ctx, &models.SearchUserOptions{
		Actor:       ctx.User,
		Type:        models.UserTypeIndividual,
		ListOptions: models.ListOptions{PageSize: setting.UI.ExplorePagingNum},
		IsActive:    util.OptionalBoolTrue,
		Visible:     []structs.VisibleType{structs.VisibleTypePublic, structs.VisibleTypeLimited, structs.VisibleTypePrivate},
	}, tplExploreUsers)
}
