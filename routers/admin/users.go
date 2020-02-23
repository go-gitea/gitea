// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/password"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers"
	"code.gitea.io/gitea/services/mailer"

	"github.com/unknwon/com"
)

const (
	tplUsers    base.TplName = "admin/user/list"
	tplUserNew  base.TplName = "admin/user/new"
	tplUserEdit base.TplName = "admin/user/edit"
)

// Users show all the users
func Users(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.users")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminUsers"] = true

	routers.RenderUserSearch(ctx, &models.SearchUserOptions{
		Type:          models.UserTypeIndividual,
		PageSize:      setting.UI.Admin.UserPagingNum,
		SearchByEmail: true,
	}, tplUsers)
}

// NewUser render adding a new user page
func NewUser(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.users.new_account")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminUsers"] = true

	ctx.Data["login_type"] = "0-0"

	sources, err := models.LoginSources()
	if err != nil {
		ctx.ServerError("LoginSources", err)
		return
	}
	ctx.Data["Sources"] = sources

	ctx.Data["CanSendEmail"] = setting.MailService != nil
	ctx.HTML(200, tplUserNew)
}

// NewUserPost response for adding a new user
func NewUserPost(ctx *context.Context, form auth.AdminCreateUserForm) {
	ctx.Data["Title"] = ctx.Tr("admin.users.new_account")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminUsers"] = true

	sources, err := models.LoginSources()
	if err != nil {
		ctx.ServerError("LoginSources", err)
		return
	}
	ctx.Data["Sources"] = sources

	ctx.Data["CanSendEmail"] = setting.MailService != nil

	if ctx.HasError() {
		ctx.HTML(200, tplUserNew)
		return
	}

	u := &models.User{
		Name:      form.UserName,
		Email:     form.Email,
		Passwd:    form.Password,
		IsActive:  true,
		LoginType: models.LoginPlain,
	}

	if len(form.LoginType) > 0 {
		fields := strings.Split(form.LoginType, "-")
		if len(fields) == 2 {
			u.LoginType = models.LoginType(com.StrTo(fields[0]).MustInt())
			u.LoginSource = com.StrTo(fields[1]).MustInt64()
			u.LoginName = form.LoginName
		}
	}
	if u.LoginType == models.LoginNoType || u.LoginType == models.LoginPlain {
		if len(form.Password) < setting.MinPasswordLength {
			ctx.Data["Err_Password"] = true
			ctx.RenderWithErr(ctx.Tr("auth.password_too_short", setting.MinPasswordLength), tplUserNew, &form)
			return
		}
		if !password.IsComplexEnough(form.Password) {
			ctx.Data["Err_Password"] = true
			ctx.RenderWithErr(password.BuildComplexityError(ctx), tplUserNew, &form)
			return
		}
		u.MustChangePassword = form.MustChangePassword
	}
	if err := models.CreateUser(u); err != nil {
		switch {
		case models.IsErrUserAlreadyExist(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("form.username_been_taken"), tplUserNew, &form)
		case models.IsErrEmailAlreadyUsed(err):
			ctx.Data["Err_Email"] = true
			ctx.RenderWithErr(ctx.Tr("form.email_been_used"), tplUserNew, &form)
		case models.IsErrNameReserved(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("user.form.name_reserved", err.(models.ErrNameReserved).Name), tplUserNew, &form)
		case models.IsErrNamePatternNotAllowed(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("user.form.name_pattern_not_allowed", err.(models.ErrNamePatternNotAllowed).Pattern), tplUserNew, &form)
		case models.IsErrNameCharsNotAllowed(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("user.form.name_chars_not_allowed", err.(models.ErrNameCharsNotAllowed).Name), tplUserNew, &form)
		default:
			ctx.ServerError("CreateUser", err)
		}
		return
	}
	log.Trace("Account created by admin (%s): %s", ctx.User.Name, u.Name)

	// Send email notification.
	if form.SendNotify {
		mailer.SendRegisterNotifyMail(ctx.Locale, u)
	}

	ctx.Flash.Success(ctx.Tr("admin.users.new_success", u.Name))
	ctx.Redirect(setting.AppSubURL + "/admin/users/" + com.ToStr(u.ID))
}

func prepareUserInfo(ctx *context.Context) *models.User {
	u, err := models.GetUserByID(ctx.ParamsInt64(":userid"))
	if err != nil {
		ctx.ServerError("GetUserByID", err)
		return nil
	}
	ctx.Data["User"] = u

	if u.LoginSource > 0 {
		ctx.Data["LoginSource"], err = models.GetLoginSourceByID(u.LoginSource)
		if err != nil {
			ctx.ServerError("GetLoginSourceByID", err)
			return nil
		}
	} else {
		ctx.Data["LoginSource"] = &models.LoginSource{}
	}

	sources, err := models.LoginSources()
	if err != nil {
		ctx.ServerError("LoginSources", err)
		return nil
	}
	ctx.Data["Sources"] = sources

	return u
}

// EditUser show editting user page
func EditUser(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.users.edit_account")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminUsers"] = true
	ctx.Data["DisableRegularOrgCreation"] = setting.Admin.DisableRegularOrgCreation

	prepareUserInfo(ctx)
	if ctx.Written() {
		return
	}

	ctx.HTML(200, tplUserEdit)
}

// EditUserPost response for editting user
func EditUserPost(ctx *context.Context, form auth.AdminEditUserForm) {
	ctx.Data["Title"] = ctx.Tr("admin.users.edit_account")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminUsers"] = true

	u := prepareUserInfo(ctx)
	if ctx.Written() {
		return
	}

	if ctx.HasError() {
		ctx.HTML(200, tplUserEdit)
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
		var err error
		if len(form.Password) < setting.MinPasswordLength {
			ctx.Data["Err_Password"] = true
			ctx.RenderWithErr(ctx.Tr("auth.password_too_short", setting.MinPasswordLength), tplUserEdit, &form)
			return
		}
		if !password.IsComplexEnough(form.Password) {
			ctx.RenderWithErr(password.BuildComplexityError(ctx), tplUserEdit, &form)
			return
		}
		if u.Salt, err = models.GetUserSalt(); err != nil {
			ctx.ServerError("UpdateUser", err)
			return
		}
		u.HashPassword(form.Password)
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
	u.AllowCreateOrganization = form.AllowCreateOrganization
	u.ProhibitLogin = form.ProhibitLogin

	if err := models.UpdateUser(u); err != nil {
		if models.IsErrEmailAlreadyUsed(err) {
			ctx.Data["Err_Email"] = true
			ctx.RenderWithErr(ctx.Tr("form.email_been_used"), tplUserEdit, &form)
		} else {
			ctx.ServerError("UpdateUser", err)
		}
		return
	}
	log.Trace("Account profile updated by admin (%s): %s", ctx.User.Name, u.Name)

	ctx.Flash.Success(ctx.Tr("admin.users.update_profile_success"))
	ctx.Redirect(setting.AppSubURL + "/admin/users/" + ctx.Params(":userid"))
}

// DeleteUser response for deleting a user
func DeleteUser(ctx *context.Context) {
	u, err := models.GetUserByID(ctx.ParamsInt64(":userid"))
	if err != nil {
		ctx.ServerError("GetUserByID", err)
		return
	}

	if err = models.DeleteUser(u); err != nil {
		switch {
		case models.IsErrUserOwnRepos(err):
			ctx.Flash.Error(ctx.Tr("admin.users.still_own_repo"))
			ctx.JSON(200, map[string]interface{}{
				"redirect": setting.AppSubURL + "/admin/users/" + ctx.Params(":userid"),
			})
		case models.IsErrUserHasOrgs(err):
			ctx.Flash.Error(ctx.Tr("admin.users.still_has_org"))
			ctx.JSON(200, map[string]interface{}{
				"redirect": setting.AppSubURL + "/admin/users/" + ctx.Params(":userid"),
			})
		default:
			ctx.ServerError("DeleteUser", err)
		}
		return
	}
	log.Trace("Account deleted by admin (%s): %s", ctx.User.Name, u.Name)

	ctx.Flash.Success(ctx.Tr("admin.users.deletion_success"))
	ctx.JSON(200, map[string]interface{}{
		"redirect": setting.AppSubURL + "/admin/users",
	})
}
