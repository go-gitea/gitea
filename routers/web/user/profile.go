// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"fmt"
	"net/http"
	"path"
	"strings"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/renderhelper"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/web/feed"
	"code.gitea.io/gitea/routers/web/org"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/context"
	feed_service "code.gitea.io/gitea/services/feed"
)

const (
	tplProfileBigAvatar templates.TplName = "shared/user/profile_big_avatar"
	tplFollowUnfollow   templates.TplName = "org/follow_unfollow"
)

// OwnerProfile render profile page for a user or a organization (aka, repo owner)
func OwnerProfile(ctx *context.Context) {
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
	} else {
		userProfile(ctx)
	}
}

func userProfile(ctx *context.Context) {
	// check view permissions
	if !user_model.IsUserVisibleToViewer(ctx, ctx.ContextUser, ctx.Doer) {
		ctx.NotFound("user", fmt.Errorf("%s", ctx.ContextUser.Name))
		return
	}

	ctx.Data["Title"] = ctx.ContextUser.DisplayName()
	ctx.Data["PageIsUserProfile"] = true

	// prepare heatmap data
	if setting.Service.EnableUserHeatmap {
		data, err := activities_model.GetUserHeatmapDataByUser(ctx, ctx.ContextUser, ctx.Doer)
		if err != nil {
			ctx.ServerError("GetUserHeatmapDataByUser", err)
			return
		}
		ctx.Data["HeatmapData"] = data
		ctx.Data["HeatmapTotalContributions"] = activities_model.GetTotalContributionsInHeatmap(data)
	}

	profileDbRepo, profileReadmeBlob := shared_user.FindOwnerProfileReadme(ctx, ctx.Doer)

	showPrivate := ctx.IsSigned && (ctx.Doer.IsAdmin || ctx.Doer.ID == ctx.ContextUser.ID)
	prepareUserProfileTabData(ctx, showPrivate, profileDbRepo, profileReadmeBlob)
	// call PrepareContextForProfileBigAvatar later to avoid re-querying the NumFollowers & NumFollowing
	shared_user.PrepareContextForProfileBigAvatar(ctx)
	ctx.HTML(http.StatusOK, tplProfile)
}

func prepareUserProfileTabData(ctx *context.Context, showPrivate bool, profileDbRepo *repo_model.Repository, profileReadme *git.Blob) {
	// if there is a profile readme, default to "overview" page, otherwise, default to "repositories" page
	// if there is not a profile readme, the overview tab should be treated as the repositories tab
	tab := ctx.FormString("tab")
	if tab == "" || tab == "overview" {
		if profileReadme != nil {
			tab = "overview"
		} else {
			tab = "repositories"
		}
	}
	ctx.Data["TabName"] = tab
	ctx.Data["HasUserProfileReadme"] = profileReadme != nil

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	pagingNum := setting.UI.User.RepoPagingNum
	topicOnly := ctx.FormBool("topic")
	var (
		repos   []*repo_model.Repository
		count   int64
		total   int
		orderBy db.SearchOrderBy
	)

	sortOrder := ctx.FormString("sort")
	if _, ok := repo_model.OrderByFlatMap[sortOrder]; !ok {
		sortOrder = setting.UI.ExploreDefaultSort // TODO: add new default sort order for user home?
	}
	ctx.Data["SortType"] = sortOrder
	orderBy = repo_model.OrderByFlatMap[sortOrder]

	keyword := ctx.FormTrim("q")
	ctx.Data["Keyword"] = keyword

	language := ctx.FormTrim("language")
	ctx.Data["Language"] = language

	followers, numFollowers, err := user_model.GetUserFollowers(ctx, ctx.ContextUser, ctx.Doer, db.ListOptions{
		PageSize: pagingNum,
		Page:     page,
	})
	if err != nil {
		ctx.ServerError("GetUserFollowers", err)
		return
	}
	ctx.Data["NumFollowers"] = numFollowers
	following, numFollowing, err := user_model.GetUserFollowing(ctx, ctx.ContextUser, ctx.Doer, db.ListOptions{
		PageSize: pagingNum,
		Page:     page,
	})
	if err != nil {
		ctx.ServerError("GetUserFollowing", err)
		return
	}
	ctx.Data["NumFollowing"] = numFollowing

	archived := ctx.FormOptionalBool("archived")
	ctx.Data["IsArchived"] = archived

	fork := ctx.FormOptionalBool("fork")
	ctx.Data["IsFork"] = fork

	mirror := ctx.FormOptionalBool("mirror")
	ctx.Data["IsMirror"] = mirror

	template := ctx.FormOptionalBool("template")
	ctx.Data["IsTemplate"] = template

	private := ctx.FormOptionalBool("private")
	ctx.Data["IsPrivate"] = private

	switch tab {
	case "followers":
		ctx.Data["Cards"] = followers
		total = int(numFollowers)
	case "following":
		ctx.Data["Cards"] = following
		total = int(numFollowing)
	case "activity":
		date := ctx.FormString("date")
		pagingNum = setting.UI.FeedPagingNum
		items, count, err := feed_service.GetFeeds(ctx, activities_model.GetFeedsOptions{
			RequestedUser:   ctx.ContextUser,
			Actor:           ctx.Doer,
			IncludePrivate:  showPrivate,
			OnlyPerformedBy: true,
			IncludeDeleted:  false,
			Date:            date,
			ListOptions: db.ListOptions{
				PageSize: pagingNum,
				Page:     page,
			},
		})
		if err != nil {
			ctx.ServerError("GetFeeds", err)
			return
		}
		ctx.Data["Feeds"] = items
		ctx.Data["Date"] = date

		total = int(count)
	case "stars":
		ctx.Data["PageIsProfileStarList"] = true
		repos, count, err = repo_model.SearchRepository(ctx, &repo_model.SearchRepoOptions{
			ListOptions: db.ListOptions{
				PageSize: pagingNum,
				Page:     page,
			},
			Actor:              ctx.Doer,
			Keyword:            keyword,
			OrderBy:            orderBy,
			Private:            ctx.IsSigned,
			StarredByID:        ctx.ContextUser.ID,
			Collaborate:        optional.Some(false),
			TopicOnly:          topicOnly,
			Language:           language,
			IncludeDescription: setting.UI.SearchRepoDescription,
			Archived:           archived,
			Fork:               fork,
			Mirror:             mirror,
			Template:           template,
			IsPrivate:          private,
		})
		if err != nil {
			ctx.ServerError("SearchRepository", err)
			return
		}

		total = int(count)
	case "watching":
		repos, count, err = repo_model.SearchRepository(ctx, &repo_model.SearchRepoOptions{
			ListOptions: db.ListOptions{
				PageSize: pagingNum,
				Page:     page,
			},
			Actor:              ctx.Doer,
			Keyword:            keyword,
			OrderBy:            orderBy,
			Private:            ctx.IsSigned,
			WatchedByID:        ctx.ContextUser.ID,
			Collaborate:        optional.Some(false),
			TopicOnly:          topicOnly,
			Language:           language,
			IncludeDescription: setting.UI.SearchRepoDescription,
			Archived:           archived,
			Fork:               fork,
			Mirror:             mirror,
			Template:           template,
			IsPrivate:          private,
		})
		if err != nil {
			ctx.ServerError("SearchRepository", err)
			return
		}

		total = int(count)
	case "overview":
		if bytes, err := profileReadme.GetBlobContent(setting.UI.MaxDisplayFileSize); err != nil {
			log.Error("failed to GetBlobContent: %v", err)
		} else {
			rctx := renderhelper.NewRenderContextRepoFile(ctx, profileDbRepo, renderhelper.RepoFileOptions{
				CurrentRefPath: path.Join("branch", util.PathEscapeSegments(profileDbRepo.DefaultBranch)),
			})
			if profileContent, err := markdown.RenderString(rctx, bytes); err != nil {
				log.Error("failed to RenderString: %v", err)
			} else {
				ctx.Data["ProfileReadmeContent"] = profileContent
			}
		}
	case "organizations":
		orgs, count, err := db.FindAndCount[organization.Organization](ctx, organization.FindOrgOptions{
			UserID:         ctx.ContextUser.ID,
			IncludePrivate: showPrivate,
			ListOptions: db.ListOptions{
				Page:     page,
				PageSize: pagingNum,
			},
		})
		if err != nil {
			ctx.ServerError("GetUserOrganizations", err)
			return
		}
		ctx.Data["Cards"] = orgs
		total = int(count)
	default: // default to "repositories"
		repos, count, err = repo_model.SearchRepository(ctx, &repo_model.SearchRepoOptions{
			ListOptions: db.ListOptions{
				PageSize: pagingNum,
				Page:     page,
			},
			Actor:              ctx.Doer,
			Keyword:            keyword,
			OwnerID:            ctx.ContextUser.ID,
			OrderBy:            orderBy,
			Private:            ctx.IsSigned,
			Collaborate:        optional.Some(false),
			TopicOnly:          topicOnly,
			Language:           language,
			IncludeDescription: setting.UI.SearchRepoDescription,
			Archived:           archived,
			Fork:               fork,
			Mirror:             mirror,
			Template:           template,
			IsPrivate:          private,
		})
		if err != nil {
			ctx.ServerError("SearchRepository", err)
			return
		}

		total = int(count)
	}
	ctx.Data["Repos"] = repos
	ctx.Data["Total"] = total

	err = shared_user.LoadHeaderCount(ctx)
	if err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

	pager := context.NewPagination(total, pagingNum, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager
}

// Action response for follow/unfollow user request
func Action(ctx *context.Context) {
	var err error
	switch ctx.FormString("action") {
	case "follow":
		err = user_model.FollowUser(ctx, ctx.Doer, ctx.ContextUser)
	case "unfollow":
		err = user_model.UnfollowUser(ctx, ctx.Doer.ID, ctx.ContextUser.ID)
	}

	if err != nil {
		log.Error("Failed to apply action %q: %v", ctx.FormString("action"), err)
		ctx.Error(http.StatusBadRequest, fmt.Sprintf("Action %q failed", ctx.FormString("action")))
		return
	}

	if ctx.ContextUser.IsIndividual() {
		shared_user.PrepareContextForProfileBigAvatar(ctx)
		ctx.HTML(http.StatusOK, tplProfileBigAvatar)
		return
	} else if ctx.ContextUser.IsOrganization() {
		ctx.Data["Org"] = ctx.ContextUser
		ctx.Data["IsFollowing"] = ctx.Doer != nil && user_model.IsFollowing(ctx, ctx.Doer.ID, ctx.ContextUser.ID)
		ctx.HTML(http.StatusOK, tplFollowUnfollow)
		return
	}
	log.Error("Failed to apply action %q: unsupport context user type: %s", ctx.FormString("action"), ctx.ContextUser.Type)
	ctx.Error(http.StatusBadRequest, fmt.Sprintf("Action %q failed", ctx.FormString("action")))
}
