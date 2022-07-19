// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/web/feed"
	"code.gitea.io/gitea/routers/web/org"
)

// Profile render user's profile page
func Profile(ctx *context.Context) {
	if strings.Contains(ctx.Req.Header.Get("Accept"), "application/rss+xml") {
		feed.ShowUserFeedRSS(ctx)
		return
	}
	if strings.Contains(ctx.Req.Header.Get("Accept"), "application/atom+xml") {
		feed.ShowUserFeedAtom(ctx)
		return
	}

	if ctx.ContextUser.IsOrganization() {
		org.Home(ctx)
		return
	}

	// check view permissions
	if !user_model.IsUserVisibleToViewer(ctx, ctx.ContextUser, ctx.Doer) {
		ctx.NotFound("user", fmt.Errorf(ctx.ContextUser.Name))
		return
	}

	// advertise feed via meta tag
	ctx.Data["FeedURL"] = ctx.ContextUser.HTMLURL()

	// Show OpenID URIs
	openIDs, err := user_model.GetUserOpenIDs(ctx.ContextUser.ID)
	if err != nil {
		ctx.ServerError("GetUserOpenIDs", err)
		return
	}

	var isFollowing bool
	if ctx.Doer != nil {
		isFollowing = user_model.IsFollowing(ctx.Doer.ID, ctx.ContextUser.ID)
	}

	ctx.Data["Title"] = ctx.ContextUser.DisplayName()
	ctx.Data["PageIsUserProfile"] = true
	ctx.Data["Owner"] = ctx.ContextUser
	ctx.Data["OpenIDs"] = openIDs
	ctx.Data["IsFollowing"] = isFollowing

	if setting.Service.EnableUserHeatmap {
		data, err := models.GetUserHeatmapDataByUser(ctx.ContextUser, ctx.Doer)
		if err != nil {
			ctx.ServerError("GetUserHeatmapDataByUser", err)
			return
		}
		ctx.Data["HeatmapData"] = data
	}

	if len(ctx.ContextUser.Description) != 0 {
		content, err := markdown.RenderString(&markup.RenderContext{
			URLPrefix: ctx.Repo.RepoLink,
			Metas:     map[string]string{"mode": "document"},
			GitRepo:   ctx.Repo.GitRepo,
			Ctx:       ctx,
		}, ctx.ContextUser.Description)
		if err != nil {
			ctx.ServerError("RenderString", err)
			return
		}
		ctx.Data["RenderedDescription"] = content
	}

	showPrivate := ctx.IsSigned && (ctx.Doer.IsAdmin || ctx.Doer.ID == ctx.ContextUser.ID)

	orgs, err := organization.FindOrgs(organization.FindOrgOptions{
		UserID:         ctx.ContextUser.ID,
		IncludePrivate: showPrivate,
	})
	if err != nil {
		ctx.ServerError("FindOrgs", err)
		return
	}

	ctx.Data["Orgs"] = orgs
	ctx.Data["HasOrgsVisible"] = organization.HasOrgsVisible(orgs, ctx.Doer)

	tab := ctx.FormString("tab")
	ctx.Data["TabName"] = tab

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	topicOnly := ctx.FormBool("topic")

	var (
		repos   []*repo_model.Repository
		count   int64
		total   int
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
	ctx.Data["Keyword"] = keyword

	language := ctx.FormTrim("language")
	ctx.Data["Language"] = language

	switch tab {
	case "followers":
		items, count, err := user_model.GetUserFollowers(ctx, ctx.ContextUser, ctx.Doer, db.ListOptions{
			PageSize: setting.UI.User.RepoPagingNum,
			Page:     page,
		})
		if err != nil {
			ctx.ServerError("GetUserFollowers", err)
			return
		}
		ctx.Data["Cards"] = items

		total = int(count)
	case "following":
		items, count, err := user_model.GetUserFollowing(ctx, ctx.ContextUser, ctx.Doer, db.ListOptions{
			PageSize: setting.UI.User.RepoPagingNum,
			Page:     page,
		})
		if err != nil {
			ctx.ServerError("GetUserFollowing", err)
			return
		}
		ctx.Data["Cards"] = items

		total = int(count)
	case "activity":
		ctx.Data["Feeds"], err = models.GetFeeds(ctx, models.GetFeedsOptions{
			RequestedUser:   ctx.ContextUser,
			Actor:           ctx.Doer,
			IncludePrivate:  showPrivate,
			OnlyPerformedBy: true,
			IncludeDeleted:  false,
			Date:            ctx.FormString("date"),
			ListOptions:     db.ListOptions{PageSize: setting.UI.FeedPagingNum},
		})
		if err != nil {
			ctx.ServerError("GetFeeds", err)
			return
		}
	case "stars":
		ctx.Data["PageIsProfileStarList"] = true
		repos, count, err = repo_model.SearchRepository(&repo_model.SearchRepoOptions{
			ListOptions: db.ListOptions{
				PageSize: setting.UI.User.RepoPagingNum,
				Page:     page,
			},
			Actor:              ctx.Doer,
			Keyword:            keyword,
			OrderBy:            orderBy,
			Private:            ctx.IsSigned,
			StarredByID:        ctx.ContextUser.ID,
			Collaborate:        util.OptionalBoolFalse,
			TopicOnly:          topicOnly,
			Language:           language,
			IncludeDescription: setting.UI.SearchRepoDescription,
		})
		if err != nil {
			ctx.ServerError("SearchRepository", err)
			return
		}

		total = int(count)
	case "projects":
		ctx.Data["OpenProjects"], _, err = project_model.GetProjects(ctx, project_model.SearchOptions{
			Page:     -1,
			IsClosed: util.OptionalBoolFalse,
			Type:     project_model.TypeIndividual,
		})
		if err != nil {
			ctx.ServerError("GetProjects", err)
			return
		}
	case "watching":
		repos, count, err = repo_model.SearchRepository(&repo_model.SearchRepoOptions{
			ListOptions: db.ListOptions{
				PageSize: setting.UI.User.RepoPagingNum,
				Page:     page,
			},
			Actor:              ctx.Doer,
			Keyword:            keyword,
			OrderBy:            orderBy,
			Private:            ctx.IsSigned,
			WatchedByID:        ctx.ContextUser.ID,
			Collaborate:        util.OptionalBoolFalse,
			TopicOnly:          topicOnly,
			Language:           language,
			IncludeDescription: setting.UI.SearchRepoDescription,
		})
		if err != nil {
			ctx.ServerError("SearchRepository", err)
			return
		}

		total = int(count)
	default:
		repos, count, err = repo_model.SearchRepository(&repo_model.SearchRepoOptions{
			ListOptions: db.ListOptions{
				PageSize: setting.UI.User.RepoPagingNum,
				Page:     page,
			},
			Actor:              ctx.Doer,
			Keyword:            keyword,
			OwnerID:            ctx.ContextUser.ID,
			OrderBy:            orderBy,
			Private:            ctx.IsSigned,
			Collaborate:        util.OptionalBoolFalse,
			TopicOnly:          topicOnly,
			Language:           language,
			IncludeDescription: setting.UI.SearchRepoDescription,
		})
		if err != nil {
			ctx.ServerError("SearchRepository", err)
			return
		}

		total = int(count)
	}
	ctx.Data["Repos"] = repos
	ctx.Data["Total"] = total

	pager := context.NewPagination(total, setting.UI.User.RepoPagingNum, page, 5)
	pager.SetDefaultParams(ctx)
	pager.AddParam(ctx, "tab", "TabName")
	if tab != "followers" && tab != "following" && tab != "activity" && tab != "projects" {
		pager.AddParam(ctx, "language", "Language")
	}
	ctx.Data["Page"] = pager
	ctx.Data["IsPackageEnabled"] = setting.Packages.Enabled

	ctx.Data["ShowUserEmail"] = len(ctx.ContextUser.Email) > 0 && ctx.IsSigned && (!ctx.ContextUser.KeepEmailPrivate || ctx.ContextUser.ID == ctx.Doer.ID)

	ctx.HTML(http.StatusOK, tplProfile)
}

// Action response for follow/unfollow user request
func Action(ctx *context.Context) {
	var err error
	switch ctx.FormString("action") {
	case "follow":
		err = user_model.FollowUser(ctx.Doer.ID, ctx.ContextUser.ID)
	case "unfollow":
		err = user_model.UnfollowUser(ctx.Doer.ID, ctx.ContextUser.ID)
	}

	if err != nil {
		ctx.ServerError(fmt.Sprintf("Action (%s)", ctx.FormString("action")), err)
		return
	}
	// FIXME: We should check this URL and make sure that it's a valid Gitea URL
	ctx.RedirectToFirst(ctx.FormString("redirect_to"), ctx.ContextUser.HomeLink())
}
