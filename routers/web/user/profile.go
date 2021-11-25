// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"fmt"
	"net/http"
	"path"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/web/feed"
	"code.gitea.io/gitea/routers/web/org"
)

// GetUserByName get user by name
func GetUserByName(ctx *context.Context, name string) *user_model.User {
	user, err := user_model.GetUserByName(name)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			if redirectUserID, err := user_model.LookupUserRedirect(name); err == nil {
				context.RedirectToUser(ctx, name, redirectUserID)
			} else {
				ctx.NotFound("GetUserByName", err)
			}
		} else {
			ctx.ServerError("GetUserByName", err)
		}
		return nil
	}
	return user
}

// GetUserByParams returns user whose name is presented in URL paramenter.
func GetUserByParams(ctx *context.Context) *user_model.User {
	return GetUserByName(ctx, ctx.Params(":username"))
}

// Profile render user's profile page
func Profile(ctx *context.Context) {
	uname := ctx.Params(":username")

	// Special handle for FireFox requests favicon.ico.
	if uname == "favicon.ico" {
		ctx.ServeFile(path.Join(setting.StaticRootPath, "public/img/favicon.png"))
		return
	}

	if strings.HasSuffix(uname, ".png") {
		ctx.Error(http.StatusNotFound)
		return
	}

	isShowKeys := false
	if strings.HasSuffix(uname, ".keys") {
		isShowKeys = true
		uname = strings.TrimSuffix(uname, ".keys")
	}

	isShowGPG := false
	if strings.HasSuffix(uname, ".gpg") {
		isShowGPG = true
		uname = strings.TrimSuffix(uname, ".gpg")
	}

	showFeedType := ""
	if strings.HasSuffix(uname, ".rss") {
		showFeedType = "rss"
		uname = strings.TrimSuffix(uname, ".rss")
	} else if strings.Contains(ctx.Req.Header.Get("Accept"), "application/rss+xml") {
		showFeedType = "rss"
	}
	if strings.HasSuffix(uname, ".atom") {
		showFeedType = "atom"
		uname = strings.TrimSuffix(uname, ".atom")
	} else if strings.Contains(ctx.Req.Header.Get("Accept"), "application/atom+xml") {
		showFeedType = "atom"
	}

	ctxUser := GetUserByName(ctx, uname)
	if ctx.Written() {
		return
	}

	if ctxUser.IsOrganization() {
		/*
			// TODO: enable after rss.RetrieveFeeds() do handle org correctly
			// Show Org RSS feed
			if len(showFeedType) != 0 {
				rss.ShowUserFeed(ctx, ctxUser, showFeedType)
				return
			}
		*/

		org.Home(ctx)
		return
	}

	// check view permissions
	if !models.IsUserVisibleToViewer(ctxUser, ctx.User) {
		ctx.NotFound("user", fmt.Errorf(uname))
		return
	}

	// Show SSH keys.
	if isShowKeys {
		ShowSSHKeys(ctx, ctxUser.ID)
		return
	}

	// Show GPG keys.
	if isShowGPG {
		ShowGPGKeys(ctx, ctxUser.ID)
		return
	}

	// Show User RSS feed
	if len(showFeedType) != 0 {
		feed.ShowUserFeed(ctx, ctxUser, showFeedType)
		return
	}

	// Show OpenID URIs
	openIDs, err := user_model.GetUserOpenIDs(ctxUser.ID)
	if err != nil {
		ctx.ServerError("GetUserOpenIDs", err)
		return
	}

	var isFollowing bool
	if ctx.User != nil && ctxUser != nil {
		isFollowing = user_model.IsFollowing(ctx.User.ID, ctxUser.ID)
	}

	ctx.Data["Title"] = ctxUser.DisplayName()
	ctx.Data["PageIsUserProfile"] = true
	ctx.Data["Owner"] = ctxUser
	ctx.Data["OpenIDs"] = openIDs
	ctx.Data["IsFollowing"] = isFollowing

	if setting.Service.EnableUserHeatmap {
		data, err := models.GetUserHeatmapDataByUser(ctxUser, ctx.User)
		if err != nil {
			ctx.ServerError("GetUserHeatmapDataByUser", err)
			return
		}
		ctx.Data["HeatmapData"] = data
	}

	if len(ctxUser.Description) != 0 {
		content, err := markdown.RenderString(&markup.RenderContext{
			URLPrefix: ctx.Repo.RepoLink,
			Metas:     map[string]string{"mode": "document"},
			GitRepo:   ctx.Repo.GitRepo,
			Ctx:       ctx,
		}, ctxUser.Description)
		if err != nil {
			ctx.ServerError("RenderString", err)
			return
		}
		ctx.Data["RenderedDescription"] = content
	}

	showPrivate := ctx.IsSigned && (ctx.User.IsAdmin || ctx.User.ID == ctxUser.ID)

	orgs, err := models.FindOrgs(models.FindOrgOptions{
		UserID:         ctxUser.ID,
		IncludePrivate: showPrivate,
	})
	if err != nil {
		ctx.ServerError("FindOrgs", err)
		return
	}

	ctx.Data["Orgs"] = orgs
	ctx.Data["HasOrgsVisible"] = models.HasOrgsVisible(orgs, ctx.User)

	tab := ctx.FormString("tab")
	ctx.Data["TabName"] = tab

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	topicOnly := ctx.FormBool("topic")

	var (
		repos   []*models.Repository
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
	switch tab {
	case "followers":
		items, err := user_model.GetUserFollowers(ctxUser, db.ListOptions{
			PageSize: setting.UI.User.RepoPagingNum,
			Page:     page,
		})
		if err != nil {
			ctx.ServerError("GetUserFollowers", err)
			return
		}
		ctx.Data["Cards"] = items

		total = ctxUser.NumFollowers
	case "following":
		items, err := user_model.GetUserFollowing(ctxUser, db.ListOptions{
			PageSize: setting.UI.User.RepoPagingNum,
			Page:     page,
		})
		if err != nil {
			ctx.ServerError("GetUserFollowing", err)
			return
		}
		ctx.Data["Cards"] = items

		total = ctxUser.NumFollowing
	case "activity":
		ctx.Data["Feeds"] = feed.RetrieveFeeds(ctx, models.GetFeedsOptions{RequestedUser: ctxUser,
			Actor:           ctx.User,
			IncludePrivate:  showPrivate,
			OnlyPerformedBy: true,
			IncludeDeleted:  false,
			Date:            ctx.FormString("date"),
		})
		if ctx.Written() {
			return
		}
	case "stars":
		ctx.Data["PageIsProfileStarList"] = true
		repos, count, err = models.SearchRepository(&models.SearchRepoOptions{
			ListOptions: db.ListOptions{
				PageSize: setting.UI.User.RepoPagingNum,
				Page:     page,
			},
			Actor:              ctx.User,
			Keyword:            keyword,
			OrderBy:            orderBy,
			Private:            ctx.IsSigned,
			StarredByID:        ctxUser.ID,
			Collaborate:        util.OptionalBoolFalse,
			TopicOnly:          topicOnly,
			IncludeDescription: setting.UI.SearchRepoDescription,
		})
		if err != nil {
			ctx.ServerError("SearchRepository", err)
			return
		}

		total = int(count)
	case "projects":
		ctx.Data["OpenProjects"], _, err = models.GetProjects(models.ProjectSearchOptions{
			Page:     -1,
			IsClosed: util.OptionalBoolFalse,
			Type:     models.ProjectTypeIndividual,
		})
		if err != nil {
			ctx.ServerError("GetProjects", err)
			return
		}
	case "watching":
		repos, count, err = models.SearchRepository(&models.SearchRepoOptions{
			ListOptions: db.ListOptions{
				PageSize: setting.UI.User.RepoPagingNum,
				Page:     page,
			},
			Actor:              ctx.User,
			Keyword:            keyword,
			OrderBy:            orderBy,
			Private:            ctx.IsSigned,
			WatchedByID:        ctxUser.ID,
			Collaborate:        util.OptionalBoolFalse,
			TopicOnly:          topicOnly,
			IncludeDescription: setting.UI.SearchRepoDescription,
		})
		if err != nil {
			ctx.ServerError("SearchRepository", err)
			return
		}

		total = int(count)
	default:
		repos, count, err = models.SearchRepository(&models.SearchRepoOptions{
			ListOptions: db.ListOptions{
				PageSize: setting.UI.User.RepoPagingNum,
				Page:     page,
			},
			Actor:              ctx.User,
			Keyword:            keyword,
			OwnerID:            ctxUser.ID,
			OrderBy:            orderBy,
			Private:            ctx.IsSigned,
			Collaborate:        util.OptionalBoolFalse,
			TopicOnly:          topicOnly,
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
	ctx.Data["Page"] = pager

	ctx.Data["ShowUserEmail"] = len(ctxUser.Email) > 0 && ctx.IsSigned && (!ctxUser.KeepEmailPrivate || ctxUser.ID == ctx.User.ID)

	ctx.HTML(http.StatusOK, tplProfile)
}

// Action response for follow/unfollow user request
func Action(ctx *context.Context) {
	u := GetUserByParams(ctx)
	if ctx.Written() {
		return
	}

	var err error
	switch ctx.Params(":action") {
	case "follow":
		err = user_model.FollowUser(ctx.User.ID, u.ID)
	case "unfollow":
		err = user_model.UnfollowUser(ctx.User.ID, u.ID)
	}

	if err != nil {
		ctx.ServerError(fmt.Sprintf("Action (%s)", ctx.Params(":action")), err)
		return
	}
	// FIXME: We should check this URL and make sure that it's a valid Gitea URL
	ctx.RedirectToFirst(ctx.FormString("redirect_to"), u.HomeLink())
}
