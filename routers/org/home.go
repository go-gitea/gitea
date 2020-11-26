// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package org

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
)

const (
	tplOrgHome base.TplName = "org/home"
)

// Home show organization home page
func Home(ctx *context.Context) {
	ctx.SetParams(":org", ctx.Params(":username"))
	context.HandleOrgAssignment(ctx)
	if ctx.Written() {
		return
	}

	org := ctx.Org.Organization

	if !models.HasOrgVisible(org, ctx.User) {
		ctx.NotFound("HasOrgVisible", nil)
		return
	}

	ctx.Data["PageIsUserProfile"] = true
	ctx.Data["Title"] = org.DisplayName()
	if len(org.Description) != 0 {
		ctx.Data["RenderedDescription"] = string(markdown.Render([]byte(org.Description), ctx.Repo.RepoLink, map[string]string{"mode": "document"}))
	}

	var orderBy models.SearchOrderBy
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
	case "moststars":
		orderBy = models.SearchOrderByStarsReverse
	case "feweststars":
		orderBy = models.SearchOrderByStars
	case "mostforks":
		orderBy = models.SearchOrderByForksReverse
	case "fewestforks":
		orderBy = models.SearchOrderByForks
	default:
		ctx.Data["SortType"] = "recentupdate"
		orderBy = models.SearchOrderByRecentUpdated
	}

	keyword := strings.Trim(ctx.Query("q"), " ")
	ctx.Data["Keyword"] = keyword

	page := ctx.QueryInt("page")
	if page <= 0 {
		page = 1
	}

	var (
		repos []*models.Repository
		count int64
		err   error
	)
	repos, count, err = models.SearchRepository(&models.SearchRepoOptions{
		ListOptions: models.ListOptions{
			PageSize: setting.UI.User.RepoPagingNum,
			Page:     page,
		},
		Keyword:            keyword,
		OwnerID:            org.ID,
		OrderBy:            orderBy,
		Private:            ctx.IsSigned,
		Actor:              ctx.User,
		IncludeDescription: setting.UI.SearchRepoDescription,
	})
	if err != nil {
		ctx.ServerError("SearchRepository", err)
		return
	}

	var opts = models.FindOrgMembersOpts{
		OrgID:       org.ID,
		PublicOnly:  true,
		ListOptions: models.ListOptions{Page: 1, PageSize: 25},
	}

	if ctx.User != nil {
		isMember, err := org.IsOrgMember(ctx.User.ID)
		if err != nil {
			ctx.Error(500, "IsOrgMember")
			return
		}
		opts.PublicOnly = !isMember && !ctx.User.IsAdmin
	}

	members, _, err := models.FindOrgMembers(&opts)
	if err != nil {
		ctx.ServerError("FindOrgMembers", err)
		return
	}

	membersCount, err := models.CountOrgMembers(opts)
	if err != nil {
		ctx.ServerError("CountOrgMembers", err)
		return
	}

	pinnedRepos := make([]*models.Repository, 0, 10)
	pinnedRepoIDs, err := ctx.Org.Organization.GetPinnedRepoIDs(ctx.User)
	if err != nil {
		ctx.ServerError("GetPinnedRepos", err)
		return
	}

	pinnedRepos2 := make(models.RepositoryList, 0, 5)
	for _, pinnedRepoID := range pinnedRepoIDs {
		has := false
		for _, repo := range repos {
			if repo.ID == pinnedRepoID {
				has = true
				repo.IsPinned = true
				pinnedRepos = append(pinnedRepos, repo)
				break
			}
		}

		if !has {
			repo, err := models.GetRepositoryByID(pinnedRepoID)
			if err != nil && !models.IsErrRepoNotExist(err) {
				ctx.ServerError("GetRepositoryByID", err)
				return
			}

			if repo != nil {
				repo.IsPinned = true
				pinnedRepos2 = append(pinnedRepos2, repo)
			}
		}
	}

	if len(pinnedRepos2) > 0 {
		if err = pinnedRepos2.LoadAttributes(); err != nil {
			ctx.ServerError("pinnedRepos2.LoadAttributes()", err)
			return
		}
		pinnedRepos = append(pinnedRepos, pinnedRepos2...)
	}

	ctx.Data["PinnedRepos"] = pinnedRepos
	ctx.Data["PinnedReposNum"] = len(pinnedRepos)
	ctx.Data["CanConfigPinnedRepos"] = ctx.IsSigned && ctx.Org.IsOwner
	if ctx.IsSigned && ctx.Org.IsOwner {
		ctx.Data["ConfigPinnedReposLink"] = ctx.Org.OrgLink + "/settings/pinned_repo"
	}

	ctx.Data["Owner"] = org
	ctx.Data["Repos"] = repos
	ctx.Data["Total"] = count
	ctx.Data["MembersTotal"] = membersCount
	ctx.Data["Members"] = members
	ctx.Data["Teams"] = org.Teams

	ctx.Data["DisabledMirrors"] = setting.Repository.DisableMirrors

	pager := context.NewPagination(int(count), setting.UI.User.RepoPagingNum, page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager

	ctx.HTML(200, tplOrgHome)
}
