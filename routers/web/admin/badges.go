// Copyright 2024 The Gitea Authors.
// SPDX-License-Identifier: MIT

package admin

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/web/explore"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	user_service "code.gitea.io/gitea/services/user"
)

const (
	tplBadges     base.TplName = "admin/badge/list"
	tplBadgeNew   base.TplName = "admin/badge/new"
	tplBadgeView  base.TplName = "admin/badge/view"
	tplBadgeEdit  base.TplName = "admin/badge/edit"
	tplBadgeUsers base.TplName = "admin/badge/users"
)

// BadgeSearchDefaultAdminSort is the default sort type for admin view
const BadgeSearchDefaultAdminSort = "oldest"

// Badges show all the badges
func Badges(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.badges")
	ctx.Data["PageIsAdminBadges"] = true

	sortType := ctx.FormString("sort")
	if sortType == "" {
		sortType = BadgeSearchDefaultAdminSort
		ctx.SetFormString("sort", sortType)
	}
	ctx.PageData["adminBadgeListSearchForm"] = map[string]any{
		"SortType": sortType,
	}

	explore.RenderBadgeSearch(ctx, &user_model.SearchBadgeOptions{
		Actor: ctx.Doer,
		ListOptions: db.ListOptions{
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
	ctx.Data["Title"] = ctx.Tr("admin.badges.new_badge")
	ctx.Data["PageIsAdminBadges"] = true

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplBadgeNew)
		return
	}

	b := &user_model.Badge{
		Slug:        form.Slug,
		Description: form.Description,
		ImageURL:    form.ImageURL,
	}

	if len(form.Slug) < 1 {
		ctx.Data["Err_Slug"] = true
		ctx.RenderWithErr(ctx.Tr("admin.badges.must_fill"), tplBadgeNew, &form)
		return
	}

	if err := user_model.CreateBadge(ctx, b); err != nil {
		switch {
		default:
			ctx.ServerError("CreateBadge", err)
		}
		return
	}

	log.Trace("Badge created by admin (%s): %s", ctx.Doer.Name, b.Slug)

	ctx.Flash.Success(ctx.Tr("admin.badges.new_success", b.Slug))
	ctx.Redirect(setting.AppSubURL + "/admin/badges/" + strconv.FormatInt(b.ID, 10))
}

func prepareBadgeInfo(ctx *context.Context) *user_model.Badge {
	b, err := user_model.GetBadge(ctx, ctx.PathParam(":badge_slug"))
	if err != nil {
		if user_model.IsErrBadgeNotExist(err) {
			ctx.Redirect(setting.AppSubURL + "/admin/badges")
		} else {
			ctx.ServerError("GetBadge", err)
		}
		return nil
	}
	ctx.Data["Badge"] = b

	users, count, err := user_model.GetBadgeUsers(ctx, b)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.Redirect(setting.AppSubURL + "/admin/badges")
		} else {
			ctx.ServerError("GetBadgeUsers", err)
		}
		return nil
	}
	ctx.Data["Users"] = users
	ctx.Data["UsersTotal"] = int(count)

	return b
}

func ViewBadge(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.badges.details")
	ctx.Data["PageIsAdminBadges"] = true

	prepareBadgeInfo(ctx)
	if ctx.Written() {
		return
	}

	ctx.HTML(http.StatusOK, tplBadgeView)
}

// EditBadge show editing badge page
func EditBadge(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.badges.edit_badges")
	ctx.Data["PageIsAdminBadges"] = true
	prepareBadgeInfo(ctx)
	if ctx.Written() {
		return
	}

	ctx.HTML(http.StatusOK, tplBadgeEdit)
}

// EditBadgePost response for editing badge
func EditBadgePost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.badges.edit_badges")
	ctx.Data["PageIsAdminBadges"] = true
	b := prepareBadgeInfo(ctx)
	if ctx.Written() {
		return
	}

	form := web.GetForm(ctx).(*forms.AdminCreateBadgeForm)
	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplBadgeEdit)
		return
	}

	if form.Slug != "" {
		if err := user_service.UpdateBadge(ctx, ctx.Data["Badge"].(*user_model.Badge)); err != nil {
			switch {
			default:
				ctx.ServerError("UpdateBadge", err)
			}
			return
		}
	}

	b.ImageURL = form.ImageURL
	b.Description = form.Description

	if err := user_model.UpdateBadge(ctx, ctx.Data["Badge"].(*user_model.Badge)); err != nil {
		ctx.ServerError("UpdateBadge", err)
		return
	}

	log.Trace("Badge updated by admin (%s): %s", ctx.Doer.Name, b.Slug)

	ctx.Flash.Success(ctx.Tr("admin.badges.update_success"))
	ctx.Redirect(setting.AppSubURL + "/admin/badges/" + url.PathEscape(ctx.PathParam(":badge_slug")))
}

// DeleteBadge response for deleting a badge
func DeleteBadge(ctx *context.Context) {
	b, err := user_model.GetBadge(ctx, ctx.PathParam(":badge_slug"))
	if err != nil {
		ctx.ServerError("GetBadge", err)
		return
	}

	if err = user_service.DeleteBadge(ctx, b); err != nil {
		ctx.ServerError("DeleteBadge", err)
		return
	}

	log.Trace("Badge deleted by admin (%s): %s", ctx.Doer.Name, b.Slug)

	ctx.Flash.Success(ctx.Tr("admin.badges.deletion_success"))
	ctx.Redirect(setting.AppSubURL + "/admin/badges")
}

func BadgeUsers(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.badges.users_with_badge", ctx.PathParam(":badge_slug"))
	ctx.Data["PageIsAdminBadges"] = true

	users, _, err := user_model.GetBadgeUsers(ctx, &user_model.Badge{Slug: ctx.PathParam(":badge_slug")})
	if err != nil {
		ctx.ServerError("GetBadgeUsers", err)
		return
	}

	ctx.Data["Users"] = users

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

	if err = user_model.AddUserBadge(ctx, u, &user_model.Badge{Slug: ctx.PathParam(":badge_slug")}); err != nil {
		if user_model.IsErrBadgeNotExist(err) {
			ctx.Flash.Error(ctx.Tr("admin.badges.not_found"))
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
	user, err := user_model.GetUserByID(ctx, ctx.FormInt64("id"))
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.Flash.Error(ctx.Tr("form.user_not_exist"))
		} else {
			ctx.ServerError("GetUserByName", err)
			return
		}
	}
	if err := user_model.RemoveUserBadge(ctx, user, &user_model.Badge{Slug: ctx.PathParam(":badge_slug")}); err == nil {
		ctx.Flash.Success(ctx.Tr("admin.badges.user_remove_success"))
	} else {
		ctx.Flash.Error("DeleteUser: " + err.Error())
		return
	}

	ctx.JSONRedirect(fmt.Sprintf("%s/admin/badges/%s/users", setting.AppSubURL, ctx.PathParam(":badge_slug")))
}
