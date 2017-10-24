// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routers

import (
	"bytes"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/user"

	"github.com/Unknwon/paginater"
)

const (
	// tplHome home page template
	tplHome base.TplName = "home"
	// tplExploreRepos explore repositories page template
	tplExploreRepos base.TplName = "explore/repos"
	// tplExploreUsers explore users page template
	tplExploreUsers base.TplName = "explore/users"
	// tplExploreOrganizations explore organizations page template
	tplExploreOrganizations base.TplName = "explore/organizations"
)

// Home render home page
func Home(ctx *context.Context) {
	if ctx.IsSigned {
		if !ctx.User.IsActive && setting.Service.RegisterEmailConfirm {
			ctx.Data["Title"] = ctx.Tr("auth.active_your_account")
			ctx.HTML(200, user.TplActivate)
		} else {
			user.Dashboard(ctx)
		}
		return
	}

	// Check auto-login.
	uname := ctx.GetCookie(setting.CookieUserName)
	if len(uname) != 0 {
		ctx.Redirect(setting.AppSubURL + "/user/login")
		return
	}

	ctx.Data["PageIsHome"] = true
	ctx.HTML(200, tplHome)
}

// RepoSearchOptions when calling search repositories
type RepoSearchOptions struct {
	OwnerID  int64
	Private  bool
	PageSize int
	TplName  base.TplName
}

var (
	nullByte = []byte{0x00}
)

func isKeywordValid(keyword string) bool {
	return !bytes.Contains([]byte(keyword), nullByte)
}

// RenderRepoSearch render repositories search page
func RenderRepoSearch(ctx *context.Context, opts *RepoSearchOptions) {
	page := ctx.QueryInt("page")
	if page <= 0 {
		page = 1
	}

	var (
		repos   []*models.Repository
		count   int64
		err     error
		orderBy models.SearchOrderBy
	)

	ctx.Data["SortType"] = ctx.Query("sort")
	switch ctx.Query("sort") {
	case "newest":
		orderBy = models.SearchOrderByNewest
	case "oldest":
		orderBy = models.SearchOrderByOldest
	case "recentupdate":
		orderBy = models.SearchOrderByRecentUpdated
	case "leastupdate":
		orderBy = models.SearchOrderByLeastUpdated
	case "reversealphabetically":
		orderBy = models.SearchOrderByAlphabeticallyReverse
	case "alphabetically":
		orderBy = models.SearchOrderByAlphabetically
	case "reversesize":
		orderBy = models.SearchOrderBySizeReverse
	case "size":
		orderBy = models.SearchOrderBySize
	default:
		ctx.Data["SortType"] = "recentupdate"
		orderBy = models.SearchOrderByRecentUpdated
	}

	keyword := strings.Trim(ctx.Query("q"), " ")

	repos, count, err = models.SearchRepositoryByName(&models.SearchRepoOptions{
		Page:        page,
		PageSize:    opts.PageSize,
		OrderBy:     orderBy,
		Private:     opts.Private,
		Keyword:     keyword,
		OwnerID:     opts.OwnerID,
		Collaborate: true,
		AllPublic:   true,
	})
	if err != nil {
		ctx.Handle(500, "SearchRepositoryByName", err)
		return
	}
	ctx.Data["Keyword"] = keyword
	ctx.Data["Total"] = count
	ctx.Data["Page"] = paginater.New(int(count), opts.PageSize, page, 5)
	ctx.Data["Repos"] = repos

	ctx.HTML(200, opts.TplName)
}

// ExploreRepos render explore repositories page
func ExploreRepos(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("explore")
	ctx.Data["PageIsExplore"] = true
	ctx.Data["PageIsExploreRepositories"] = true

	var ownerID int64
	if ctx.User != nil && !ctx.User.IsAdmin {
		ownerID = ctx.User.ID
	}

	RenderRepoSearch(ctx, &RepoSearchOptions{
		PageSize: setting.UI.ExplorePagingNum,
		OwnerID:  ownerID,
		Private:  ctx.User != nil,
		TplName:  tplExploreRepos,
	})
}

// RenderUserSearch render user search page
func RenderUserSearch(ctx *context.Context, opts *models.SearchUserOptions, tplName base.TplName) {
	opts.Page = ctx.QueryInt("page")
	if opts.Page <= 1 {
		opts.Page = 1
	}

	var (
		users   []*models.User
		count   int64
		err     error
		orderBy string
	)

	ctx.Data["SortType"] = ctx.Query("sort")
	switch ctx.Query("sort") {
	case "newest":
		orderBy = "id DESC"
	case "oldest":
		orderBy = "id ASC"
	case "recentupdate":
		orderBy = "updated_unix DESC"
	case "leastupdate":
		orderBy = "updated_unix ASC"
	case "reversealphabetically":
		orderBy = "name DESC"
	case "alphabetically":
		orderBy = "name ASC"
	default:
		ctx.Data["SortType"] = "alphabetically"
		orderBy = "name ASC"
	}

	opts.Keyword = strings.Trim(ctx.Query("q"), " ")
	opts.OrderBy = orderBy
	if len(opts.Keyword) == 0 || isKeywordValid(opts.Keyword) {
		users, count, err = models.SearchUsers(opts)
		if err != nil {
			ctx.Handle(500, "SearchUsers", err)
			return
		}
	}
	ctx.Data["Keyword"] = opts.Keyword
	ctx.Data["Total"] = count
	ctx.Data["Page"] = paginater.New(int(count), opts.PageSize, opts.Page, 5)
	ctx.Data["Users"] = users
	ctx.Data["ShowUserEmail"] = setting.UI.ShowUserEmail

	ctx.HTML(200, tplName)
}

// ExploreUsers render explore users page
func ExploreUsers(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("explore")
	ctx.Data["PageIsExplore"] = true
	ctx.Data["PageIsExploreUsers"] = true

	RenderUserSearch(ctx, &models.SearchUserOptions{
		Type:     models.UserTypeIndividual,
		PageSize: setting.UI.ExplorePagingNum,
		IsActive: util.OptionalBoolTrue,
	}, tplExploreUsers)
}

// ExploreOrganizations render explore organizations page
func ExploreOrganizations(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("explore")
	ctx.Data["PageIsExplore"] = true
	ctx.Data["PageIsExploreOrganizations"] = true

	RenderUserSearch(ctx, &models.SearchUserOptions{
		Type:     models.UserTypeOrganization,
		PageSize: setting.UI.ExplorePagingNum,
	}, tplExploreOrganizations)
}

// NotFound render 404 page
func NotFound(ctx *context.Context) {
	ctx.Data["Title"] = "Page Not Found"
	ctx.Handle(404, "home.NotFound", nil)
}
