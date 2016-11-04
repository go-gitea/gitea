// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"strings"

	"github.com/Unknwon/com"

	"github.com/go-gitea/gitea/models"
	"github.com/go-gitea/gitea/modules/auth"
	"github.com/go-gitea/gitea/modules/base"
	"github.com/go-gitea/gitea/modules/context"
	"github.com/go-gitea/gitea/modules/log"
	"github.com/go-gitea/gitea/modules/setting"
	"github.com/go-gitea/gitea/routers"
)

const (
	USERS     base.TplName = "admin/user/list"
	USER_NEW  base.TplName = "admin/user/new"
	USER_EDIT base.TplName = "admin/user/edit"
)

func Users(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.users")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminUsers"] = true

	routers.RenderUserSearch(ctx, &routers.UserSearchOptions{
		Type:     models.USER_TYPE_INDIVIDUAL,
		Counter:  models.CountUsers,
		Ranger:   models.Users,
		PageSize: setting.UI.Admin.UserPagingNum,
		OrderBy:  "id ASC",
		TplName:  USERS,
	})
}

func NewUser(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.users.new_account")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminUsers"] = true

	ctx.Data["login_type"] = "0-0"

	sources, err := models.LoginSources()
	if err != nil {
		ctx.Handle(500, "LoginSources", err)
		return
	}
	ctx.Data["Sources"] = sources

	ctx.Data["CanSendEmail"] = setting.MailService != nil
	ctx.HTML(200, USER_NEW)
}

func NewUserPost(ctx *context.Context, form auth.AdminCrateUserForm) {
	ctx.Data["Title"] = ctx.Tr("admin.users.new_account")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminUsers"] = true

	sources, err := models.LoginSources()
	if err != nil {
		ctx.Handle(500, "LoginSources", err)
		return
	}
	ctx.Data["Sources"] = sources

	ctx.Data["CanSendEmail"] = setting.MailService != nil

	if ctx.HasError() {
		ctx.HTML(200, USER_NEW)
		return
	}

	u := &models.User{
		Name:      form.UserName,
		Email:     form.Email,
		Passwd:    form.Password,
		IsActive:  true,
		LoginType: models.LOGIN_PLAIN,
	}

	if len(form.LoginType) > 0 {
		fields := strings.Split(form.LoginType, "-")
		if len(fields) == 2 {
			u.LoginType = models.LoginType(com.StrTo(fields[0]).MustInt())
			u.LoginSource = com.StrTo(fields[1]).MustInt64()
			u.LoginName = form.LoginName
		}
	}

	if err := models.CreateUser(u); err != nil {
		switch {
		case models.IsErrUserAlreadyExist(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("form.username_been_taken"), USER_NEW, &form)
		case models.IsErrEmailAlreadyUsed(err):
			ctx.Data["Err_Email"] = true
			ctx.RenderWithErr(ctx.Tr("form.email_been_used"), USER_NEW, &form)
		case models.IsErrNameReserved(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("user.form.name_reserved", err.(models.ErrNameReserved).Name), USER_NEW, &form)
		case models.IsErrNamePatternNotAllowed(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("user.form.name_pattern_not_allowed", err.(models.ErrNamePatternNotAllowed).Pattern), USER_NEW, &form)
		default:
			ctx.Handle(500, "CreateUser", err)
		}
		return
	}
	log.Trace("Account created by admin (%s): %s", ctx.User.Name, u.Name)

	// Send email notification.
	if form.SendNotify && setting.MailService != nil {
		models.SendRegisterNotifyMail(ctx.Context, u)
	}

	ctx.Flash.Success(ctx.Tr("admin.users.new_success", u.Name))
	ctx.Redirect(setting.AppSubUrl + "/admin/users/" + com.ToStr(u.ID))
}

func prepareUserInfo(ctx *context.Context) *models.User {
	u, err := models.GetUserByID(ctx.ParamsInt64(":userid"))
	if err != nil {
		ctx.Handle(500, "GetUserByID", err)
		return nil
	}
	ctx.Data["User"] = u

	if u.LoginSource > 0 {
		ctx.Data["LoginSource"], err = models.GetLoginSourceByID(u.LoginSource)
		if err != nil {
			ctx.Handle(500, "GetLoginSourceByID", err)
			return nil
		}
	} else {
		ctx.Data["LoginSource"] = &models.LoginSource{}
	}

	sources, err := models.LoginSources()
	if err != nil {
		ctx.Handle(500, "LoginSources", err)
		return nil
	}
	ctx.Data["Sources"] = sources

	return u
}

func EditUser(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.users.edit_account")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminUsers"] = true

	prepareUserInfo(ctx)
	if ctx.Written() {
		return
	}

	ctx.HTML(200, USER_EDIT)
}

func EditUserPost(ctx *context.Context, form auth.AdminEditUserForm) {
	ctx.Data["Title"] = ctx.Tr("admin.users.edit_account")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminUsers"] = true

	u := prepareUserInfo(ctx)
	if ctx.Written() {
		return
	}

	if ctx.HasError() {
		ctx.HTML(200, USER_EDIT)
		return
	}

	fields := strings.Split(form.LoginType, "-")
	if len(fields) == 2 {
		loginType := models.LoginType(com.StrTo(fields[0]).MustInt())
		loginSource := com.StrTo(fields[1]).MustInt64()

		if u.LoginSource != loginSource {
			u.LoginSource = loginSource
			u.LoginType = loginType
		}
	}

	if len(form.Password) > 0 {
		u.Passwd = form.Password
		u.Salt = models.GetUserSalt()
		u.EncodePasswd()
	}

	u.LoginName = form.LoginName
	u.FullName = form.FullName
	u.Email = form.Email
	u.Website = form.Website
	u.Location = form.Location
	u.MaxRepoCreation = form.MaxRepoCreation
	u.IsActive = form.Active
	u.IsAdmin = form.Admin
	u.AllowGitHook = form.AllowGitHook
	u.AllowImportLocal = form.AllowImportLocal
	u.ProhibitLogin = form.ProhibitLogin

	if err := models.UpdateUser(u); err != nil {
		if models.IsErrEmailAlreadyUsed(err) {
			ctx.Data["Err_Email"] = true
			ctx.RenderWithErr(ctx.Tr("form.email_been_used"), USER_EDIT, &form)
		} else {
			ctx.Handle(500, "UpdateUser", err)
		}
		return
	}
	log.Trace("Account profile updated by admin (%s): %s", ctx.User.Name, u.Name)

	ctx.Flash.Success(ctx.Tr("admin.users.update_profile_success"))
	ctx.Redirect(setting.AppSubUrl + "/admin/users/" + ctx.Params(":userid"))
}

func DeleteUser(ctx *context.Context) {
	u, err := models.GetUserByID(ctx.ParamsInt64(":userid"))
	if err != nil {
		ctx.Handle(500, "GetUserByID", err)
		return
	}

	if err = models.DeleteUser(u); err != nil {
		switch {
		case models.IsErrUserOwnRepos(err):
			ctx.Flash.Error(ctx.Tr("admin.users.still_own_repo"))
			ctx.JSON(200, map[string]interface{}{
				"redirect": setting.AppSubUrl + "/admin/users/" + ctx.Params(":userid"),
			})
		case models.IsErrUserHasOrgs(err):
			ctx.Flash.Error(ctx.Tr("admin.users.still_has_org"))
			ctx.JSON(200, map[string]interface{}{
				"redirect": setting.AppSubUrl + "/admin/users/" + ctx.Params(":userid"),
			})
		default:
			ctx.Handle(500, "DeleteUser", err)
		}
		return
	}
	log.Trace("Account deleted by admin (%s): %s", ctx.User.Name, u.Name)

	ctx.Flash.Success(ctx.Tr("admin.users.deletion_success"))
	ctx.JSON(200, map[string]interface{}{
		"redirect": setting.AppSubUrl + "/admin/users",
	})
}
