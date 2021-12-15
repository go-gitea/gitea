// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package explore

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

const (
	// tplExploreRepos explore repositories page template
	tplExploreRepos base.TplName = "explore/repos"
)

// RepoSearchOptions when calling search repositories
type RepoSearchOptions struct {
	OwnerID    int64
	Private    bool
	Restricted bool
	PageSize   int
	TplName    base.TplName
}

// RenderRepoSearch render repositories search page
func RenderRepoSearch(ctx *context.Context, opts *RepoSearchOptions) {
	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	var (
		repos   []*repo_model.Repository
		count   int64
		err     error
		orderBy db.SearchOrderBy
	)

	ctx.Data["SortType"] = ctx.FormString("sort")
	switch ctx.FormString("sort") {
	case "newest":
		orderBy = db.SearchOrderByNewest
	case "oldest":
		orderBy = db.SearchOrderByOldest
	case "recentupdate":
		orderBy = db.SearchOrderByRecentUpdated
	case "leastupdate":
		orderBy = db.SearchOrderByLeastUpdated
	case "reversealphabetically":
		orderBy = db.SearchOrderByAlphabeticallyReverse
	case "alphabetically":
		orderBy = db.SearchOrderByAlphabetically
	case "reversesize":
		orderBy = db.SearchOrderBySizeReverse
	case "size":
		orderBy = db.SearchOrderBySize
	case "moststars":
		orderBy = db.SearchOrderByStarsReverse
	case "feweststars":
		orderBy = db.SearchOrderByStars
	case "mostforks":
		orderBy = db.SearchOrderByForksReverse
	case "fewestforks":
		orderBy = db.SearchOrderByForks
	default:
		ctx.Data["SortType"] = "recentupdate"
		orderBy = db.SearchOrderByRecentUpdated
	}

	keyword := ctx.FormTrim("q")
	topicOnly := ctx.FormBool("topic")
	ctx.Data["TopicOnly"] = topicOnly

	repos, count, err = models.SearchRepository(&models.SearchRepoOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: opts.PageSize,
		},
		Actor:              ctx.User,
		OrderBy:            orderBy,
		Private:            opts.Private,
		Keyword:            keyword,
		OwnerID:            opts.OwnerID,
		AllPublic:          true,
		AllLimited:         true,
		TopicOnly:          topicOnly,
		IncludeDescription: setting.UI.SearchRepoDescription,
	})
	if err != nil {
		ctx.ServerError("SearchRepository", err)
		return
	}
	ctx.Data["Keyword"] = keyword
	ctx.Data["Total"] = count
	ctx.Data["Repos"] = repos
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled

	pager := context.NewPagination(int(count), opts.PageSize, page, 5)
	pager.SetDefaultParams(ctx)
	pager.AddParam(ctx, "topic", "TopicOnly")
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, opts.TplName)
}

// Repos render explore repositories page
func Repos(ctx *context.Context) {
	ctx.Data["UsersIsDisabled"] = setting.Service.Explore.DisableUsersPage
	ctx.Data["Title"] = ctx.Tr("explore")
	ctx.Data["PageIsExplore"] = true
	ctx.Data["PageIsExploreRepositories"] = true
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled

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
