// Copyright 2026 The Gitea Authors.
// SPDX-License-Identifier: MIT

package admin

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"gitea.dev/models/badges"
	"gitea.dev/models/db"
	org_model "gitea.dev/models/organization"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
)

const (
	tplBadges     templates.TplName = "admin/badge/list"
	tplBadgeNew   templates.TplName = "admin/badge/new"
	tplBadgeView  templates.TplName = "admin/badge/view"
	tplBadgeEdit  templates.TplName = "admin/badge/edit"
	tplBadgeUsers templates.TplName = "admin/badge/users"
	tplBadgeRepos templates.TplName = "admin/badge/repos"
	tplBadgeOrgs  templates.TplName = "admin/badge/orgs"
)

// BadgeSearchDefaultAdminSort is the default sort type for admin view
const BadgeSearchDefaultAdminSort = "oldest"

// Badges show all the badges
func Badges(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.badges")
	ctx.Data["PageIsAdminBadges"] = true

	RenderBadgeSearch(ctx, &badges.SearchBadgeOptions{
		ListOptions: db.ListOptions{
			Page:     max(ctx.FormInt("page"), 1),
			PageSize: setting.UI.Admin.UserPagingNum,
		},
	}, tplBadges)
}

// NewBadge render adding a new badge
func NewBadge(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.badges.new_badge")
	ctx.Data["PageIsAdminBadges"] = true

	ctx.HTML(http.StatusOK, tplBadgeNew)
}

// NewBadgePost response for adding a new badge
func NewBadgePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AdminCreateBadgeForm)

	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	b := &badges.Badge{
		Slug:        form.Slug,
		Description: form.Description,
		ImageURL:    form.ImageURL,
	}

	if err := badges.CreateBadge(ctx, b); err != nil {
		if errors.Is(err, util.ErrAlreadyExist) {
			ctx.JSONError(ctx.Tr("admin.badges.slug_been_taken"))
		} else {
			ctx.ServerError("CreateBadge", err)
		}
		return
	}

	log.Trace("Badge created by admin (%s): %s", ctx.Doer.Name, b.Slug)

	ctx.Flash.Success(ctx.Tr("admin.badges.new_success", b.Slug))
	ctx.JSONRedirect(setting.AppSubURL + "/-/admin/badges/slug/" + url.PathEscape(b.Slug))
}

func prepareBadgeInfo(ctx *context.Context) *badges.Badge {
	b, err := badges.GetBadge(ctx, ctx.PathParam("badge_slug"))
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Redirect(setting.AppSubURL + "/-/admin/badges")
		} else {
			ctx.ServerError("GetBadge", err)
		}
		return nil
	}
	ctx.Data["Badge"] = b
	return b
}

func ViewBadge(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.badges.details")
	ctx.Data["PageIsAdminBadges"] = true

	prepareBadgeInfo(ctx)
	if ctx.Written() {
		return
	}

	badge := ctx.Data["Badge"].(*badges.Badge)
	opts := &user_model.GetBadgeUsersOptions{
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: setting.UI.Admin.UserPagingNum,
		},
		BadgeSlug: badge.Slug,
	}
	users, count, err := user_model.GetBadgeUsers(ctx, opts)
	if err != nil {
		ctx.ServerError("GetBadgeUsers", err)
		return
	}
	ctx.Data["Users"] = users
	ctx.Data["UsersTotal"] = int(count)

	repoOpts := &repo_model.GetBadgeReposOptions{
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: setting.UI.Admin.UserPagingNum,
		},
		BadgeSlug: badge.Slug,
	}
	repos, repoCount, err := repo_model.GetBadgeRepos(ctx, repoOpts)
	if err != nil {
		ctx.ServerError("GetBadgeRepos", err)
		return
	}
	if err := repo_model.RepositoryList(repos).LoadOwners(ctx); err != nil {
		ctx.ServerError("LoadOwners", err)
		return
	}
	ctx.Data["Repos"] = repos
	ctx.Data["ReposTotal"] = int(repoCount)

	orgOpts := &org_model.GetBadgeOrgsOptions{
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: setting.UI.Admin.UserPagingNum,
		},
		BadgeSlug: badge.Slug,
	}
	orgs, orgCount, err := org_model.GetBadgeOrgs(ctx, orgOpts)
	if err != nil {
		ctx.ServerError("GetBadgeOrgs", err)
		return
	}
	ctx.Data["Orgs"] = orgs
	ctx.Data["OrgsTotal"] = int(orgCount)

	ctx.HTML(http.StatusOK, tplBadgeView)
}

// EditBadge show editing badge page
func EditBadge(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.badges.edit_badge")
	ctx.Data["PageIsAdminBadges"] = true
	prepareBadgeInfo(ctx)
	if ctx.Written() {
		return
	}

	ctx.HTML(http.StatusOK, tplBadgeEdit)
}

// EditBadgePost response for editing badge
func EditBadgePost(ctx *context.Context) {
	b := prepareBadgeInfo(ctx)
	if ctx.Written() {
		return
	}

	form := web.GetForm(ctx).(*forms.AdminEditBadgeForm)
	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	b.ImageURL = form.ImageURL
	b.Description = form.Description

	if err := badges.UpdateBadge(ctx, b); err != nil {
		ctx.ServerError("UpdateBadge", err)
		return
	}

	log.Trace("Badge updated by admin (%s): %s", ctx.Doer.Name, b.Slug)

	ctx.Flash.Success(ctx.Tr("admin.badges.update_success"))
	ctx.JSONRedirect(setting.AppSubURL + "/-/admin/badges/slug/" + url.PathEscape(ctx.PathParam("badge_slug")))
}

// DeleteBadge response for deleting a badge
func DeleteBadge(ctx *context.Context) {
	b, err := badges.GetBadge(ctx, ctx.PathParam("badge_slug"))
	if err != nil {
		ctx.ServerError("GetBadge", err)
		return
	}

	if err = badges.DeleteBadge(ctx, b); err != nil {
		ctx.ServerError("DeleteBadge", err)
		return
	}

	log.Trace("Badge deleted by admin (%s): %s", ctx.Doer.Name, b.Slug)

	ctx.Flash.Success(ctx.Tr("admin.badges.deletion_success"))
	ctx.Redirect(setting.AppSubURL + "/-/admin/badges")
}

//nolint:dupl // these handlers share identical structure by design
func BadgeUsers(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.badges.users_with_badge", ctx.PathParam("badge_slug"))
	ctx.Data["PageIsAdminBadges"] = true

	page := max(ctx.FormInt("page"), 1)

	badge := &badges.Badge{Slug: ctx.PathParam("badge_slug")}
	opts := &user_model.GetBadgeUsersOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: setting.UI.Admin.UserPagingNum,
		},
		BadgeSlug: badge.Slug,
	}
	users, count, err := user_model.GetBadgeUsers(ctx, opts)
	if err != nil {
		ctx.ServerError("GetBadgeUsers", err)
		return
	}

	ctx.Data["Users"] = users
	ctx.Data["Total"] = count
	ctx.Data["Page"] = context.NewPagination(count, setting.UI.Admin.UserPagingNum, page, 5)

	ctx.HTML(http.StatusOK, tplBadgeUsers)
}

// BadgeUsersPost response for actions for user badges
func BadgeUsersPost(ctx *context.Context) {
	name := strings.ToLower(ctx.FormString("user"))

	u, err := user_model.GetUserByName(ctx, name)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.Flash.Error(ctx.Tr("form.user_not_exist"))
			ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
		} else {
			ctx.ServerError("GetUserByName", err)
		}
		return
	}

	if err = user_model.AddUserBadge(ctx, u, &badges.Badge{Slug: ctx.PathParam("badge_slug")}); err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Flash.Error(ctx.Tr("admin.badges.not_found"))
			ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
		} else if errors.Is(err, util.ErrAlreadyExist) {
			ctx.Flash.Error(ctx.Tr("admin.badges.user_already_has"))
			ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
		} else {
			ctx.ServerError("AddUserBadge", err)
		}
		return
	}

	ctx.Flash.Success(ctx.Tr("admin.badges.user_add_success"))
	ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
}

// DeleteBadgeUser delete a badge from a user
func DeleteBadgeUser(ctx *context.Context) {
	badgeUsersURL := setting.AppSubURL + "/-/admin/badges/slug/" + url.PathEscape(ctx.PathParam("badge_slug")) + "/users"

	user, err := user_model.GetUserByID(ctx, ctx.FormInt64("id"))
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.Flash.Error(ctx.Tr("form.user_not_exist"))
			ctx.JSONRedirect(badgeUsersURL)
			return
		} else {
			ctx.ServerError("GetUserByID", err)
			return
		}
	}
	if err := user_model.RemoveUserBadge(ctx, user, &badges.Badge{Slug: ctx.PathParam("badge_slug")}); err == nil {
		ctx.Flash.Success(ctx.Tr("admin.badges.user_remove_success"))
	} else {
		ctx.ServerError("RemoveUserBadge", err)
		return
	}

	ctx.JSONRedirect(badgeUsersURL)
}

func BadgeRepos(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.badges.repos_with_badge", ctx.PathParam("badge_slug"))
	ctx.Data["PageIsAdminBadges"] = true

	page := max(ctx.FormInt("page"), 1)

	badge := &badges.Badge{Slug: ctx.PathParam("badge_slug")}
	opts := &repo_model.GetBadgeReposOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: setting.UI.Admin.UserPagingNum,
		},
		BadgeSlug: badge.Slug,
	}
	repos, count, err := repo_model.GetBadgeRepos(ctx, opts)
	if err != nil {
		ctx.ServerError("GetBadgeRepos", err)
		return
	}
	if err := repo_model.RepositoryList(repos).LoadOwners(ctx); err != nil {
		ctx.ServerError("LoadOwners", err)
		return
	}

	ctx.Data["Repos"] = repos
	ctx.Data["Total"] = count
	ctx.Data["Page"] = context.NewPagination(count, setting.UI.Admin.UserPagingNum, page, 5)

	ctx.HTML(http.StatusOK, tplBadgeRepos)
}

func BadgeReposPost(ctx *context.Context) {
	repoFullName := strings.ToLower(ctx.FormString("repo"))
	parts := strings.Split(repoFullName, "/")
	if len(parts) != 2 {
		ctx.Flash.Error(ctx.Tr("form.repo_not_exist"))
		ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
		return
	}
	ownerName := parts[0]
	repoName := parts[1]

	repo, err := repo_model.GetRepositoryByOwnerAndName(ctx, ownerName, repoName)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			ctx.Flash.Error(ctx.Tr("form.repo_not_exist"))
			ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
		} else {
			ctx.ServerError("GetRepositoryByOwnerAndName", err)
		}
		return
	}

	if err = repo_model.AddRepoBadge(ctx, repo, &badges.Badge{Slug: ctx.PathParam("badge_slug")}); err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Flash.Error(ctx.Tr("admin.badges.not_found"))
			ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
		} else if errors.Is(err, util.ErrAlreadyExist) {
			ctx.Flash.Error(ctx.Tr("admin.badges.repo_already_has"))
			ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
		} else {
			ctx.ServerError("AddRepoBadge", err)
		}
		return
	}

	ctx.Flash.Success(ctx.Tr("admin.badges.repo_add_success"))
	ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
}

func DeleteBadgeRepo(ctx *context.Context) {
	badgeReposURL := setting.AppSubURL + "/-/admin/badges/slug/" + url.PathEscape(ctx.PathParam("badge_slug")) + "/repos"

	repo, err := repo_model.GetRepositoryByID(ctx, ctx.FormInt64("id"))
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			ctx.Flash.Error(ctx.Tr("form.repo_not_exist"))
			ctx.JSONRedirect(badgeReposURL)
			return
		} else {
			ctx.ServerError("GetRepositoryByID", err)
			return
		}
	}
	if err := repo_model.RemoveRepoBadge(ctx, repo, &badges.Badge{Slug: ctx.PathParam("badge_slug")}); err == nil {
		ctx.Flash.Success(ctx.Tr("admin.badges.repo_remove_success"))
	} else {
		ctx.ServerError("RemoveRepoBadge", err)
		return
	}

	ctx.JSONRedirect(badgeReposURL)
}

//nolint:dupl // these handlers share identical structure by design
func BadgeOrgs(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.badges.orgs_with_badge", ctx.PathParam("badge_slug"))
	ctx.Data["PageIsAdminBadges"] = true

	page := max(ctx.FormInt("page"), 1)

	badge := &badges.Badge{Slug: ctx.PathParam("badge_slug")}
	opts := &org_model.GetBadgeOrgsOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: setting.UI.Admin.UserPagingNum,
		},
		BadgeSlug: badge.Slug,
	}
	orgs, count, err := org_model.GetBadgeOrgs(ctx, opts)
	if err != nil {
		ctx.ServerError("GetBadgeOrgs", err)
		return
	}

	ctx.Data["Orgs"] = orgs
	ctx.Data["Total"] = count
	ctx.Data["Page"] = context.NewPagination(count, setting.UI.Admin.UserPagingNum, page, 5)

	ctx.HTML(http.StatusOK, tplBadgeOrgs)
}

func BadgeOrgsPost(ctx *context.Context) {
	name := strings.ToLower(ctx.FormString("org"))

	org, err := org_model.GetOrgByName(ctx, name)
	if err != nil {
		if org_model.IsErrOrgNotExist(err) {
			ctx.Flash.Error(ctx.Tr("form.org_not_exist"))
			ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
		} else {
			ctx.ServerError("GetOrgByName", err)
		}
		return
	}

	if err = org_model.AddOrgBadge(ctx, org, &badges.Badge{Slug: ctx.PathParam("badge_slug")}); err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Flash.Error(ctx.Tr("admin.badges.not_found"))
			ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
		} else if errors.Is(err, util.ErrAlreadyExist) {
			ctx.Flash.Error(ctx.Tr("admin.badges.org_already_has"))
			ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
		} else {
			ctx.ServerError("AddOrgBadge", err)
		}
		return
	}

	ctx.Flash.Success(ctx.Tr("admin.badges.org_add_success"))
	ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
}

func DeleteBadgeOrg(ctx *context.Context) {
	badgeOrgsURL := setting.AppSubURL + "/-/admin/badges/slug/" + url.PathEscape(ctx.PathParam("badge_slug")) + "/orgs"

	org, err := org_model.GetOrgByID(ctx, ctx.FormInt64("id"))
	if err != nil {
		if org_model.IsErrOrgNotExist(err) {
			ctx.Flash.Error(ctx.Tr("form.org_not_exist"))
			ctx.JSONRedirect(badgeOrgsURL)
			return
		} else {
			ctx.ServerError("GetOrgByID", err)
			return
		}
	}
	if err := org_model.RemoveOrgBadge(ctx, org, &badges.Badge{Slug: ctx.PathParam("badge_slug")}); err == nil {
		ctx.Flash.Success(ctx.Tr("admin.badges.org_remove_success"))
	} else {
		ctx.ServerError("RemoveOrgBadge", err)
		return
	}

	ctx.JSONRedirect(badgeOrgsURL)
}

func RenderBadgeSearch(ctx *context.Context, opts *badges.SearchBadgeOptions, tplName templates.TplName) {
	var (
		badgesList []*badges.Badge
		count      int64
		err        error
		orderBy    db.SearchOrderBy
	)

	sortOrder := ctx.FormString("sort")
	if sortOrder == "" {
		sortOrder = BadgeSearchDefaultAdminSort
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
		// In case the sort type is invalid, keep admin default sorting.
		ctx.Data["SortType"] = "oldest"
		orderBy = "`badge`.id ASC"
	}

	opts.Keyword = ctx.FormTrim("q")
	opts.OrderBy = orderBy
	if len(opts.Keyword) == 0 || isKeywordValid(opts.Keyword) {
		badgesList, count, err = badges.SearchBadges(ctx, opts)
		if err != nil {
			ctx.ServerError("SearchBadges", err)
			return
		}
	}

	ctx.Data["Keyword"] = opts.Keyword
	ctx.Data["Total"] = count
	ctx.Data["Badges"] = badgesList

	pager := context.NewPagination(count, opts.PageSize, opts.Page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplName)
}
