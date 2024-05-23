// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// SPDX-License-Identifier: MIT

package admin

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/auth/password"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/web/explore"
	user_setting "code.gitea.io/gitea/routers/web/user/setting"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/mailer"
	user_service "code.gitea.io/gitea/services/user"
)

const (
	tplUsers    base.TplName = "admin/user/list"
	tplUserNew  base.TplName = "admin/user/new"
	tplUserView base.TplName = "admin/user/view"
	tplUserEdit base.TplName = "admin/user/edit"
)

// UserSearchDefaultAdminSort is the default sort type for admin view
const UserSearchDefaultAdminSort = "alphabetically"

// Users show all the users
func Users(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.users")
	ctx.Data["PageIsAdminUsers"] = true

	extraParamStrings := map[string]string{}
	statusFilterKeys := []string{"is_active", "is_admin", "is_restricted", "is_2fa_enabled", "is_prohibit_login"}
	statusFilterMap := map[string]string{}
	for _, filterKey := range statusFilterKeys {
		paramKey := "status_filter[" + filterKey + "]"
		paramVal := ctx.FormString(paramKey)
		statusFilterMap[filterKey] = paramVal
		if paramVal != "" {
			extraParamStrings[paramKey] = paramVal
		}
	}

	sortType := ctx.FormString("sort")
	if sortType == "" {
		sortType = UserSearchDefaultAdminSort
		ctx.SetFormString("sort", sortType)
	}
	ctx.PageData["adminUserListSearchForm"] = map[string]any{
		"StatusFilterMap": statusFilterMap,
		"SortType":        sortType,
	}

	explore.RenderUserSearch(ctx, &user_model.SearchUserOptions{
		Actor: ctx.Doer,
		Type:  user_model.UserTypeIndividual,
		ListOptions: db.ListOptions{
			PageSize: setting.UI.Admin.UserPagingNum,
		},
		SearchByEmail:      true,
		IsActive:           util.OptionalBoolParse(statusFilterMap["is_active"]),
		IsAdmin:            util.OptionalBoolParse(statusFilterMap["is_admin"]),
		IsRestricted:       util.OptionalBoolParse(statusFilterMap["is_restricted"]),
		IsTwoFactorEnabled: util.OptionalBoolParse(statusFilterMap["is_2fa_enabled"]),
		IsProhibitLogin:    util.OptionalBoolParse(statusFilterMap["is_prohibit_login"]),
		IncludeReserved:    true, // administrator needs to list all accounts include reserved, bot, remote ones
		ExtraParamStrings:  extraParamStrings,
	}, tplUsers)
}

// NewUser render adding a new user page
func NewUser(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.users.new_account")
	ctx.Data["PageIsAdminUsers"] = true
	ctx.Data["DefaultUserVisibilityMode"] = setting.Service.DefaultUserVisibilityMode
	ctx.Data["AllowedUserVisibilityModes"] = setting.Service.AllowedUserVisibilityModesSlice.ToVisibleTypeSlice()

	ctx.Data["login_type"] = "0-0"

	sources, err := db.Find[auth.Source](ctx, auth.FindSourcesOptions{
		IsActive: optional.Some(true),
	})
	if err != nil {
		ctx.ServerError("auth.Sources", err)
		return
	}
	ctx.Data["Sources"] = sources

	ctx.Data["CanSendEmail"] = setting.MailService != nil
	ctx.HTML(http.StatusOK, tplUserNew)
}

// NewUserPost response for adding a new user
func NewUserPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AdminCreateUserForm)
	ctx.Data["Title"] = ctx.Tr("admin.users.new_account")
	ctx.Data["PageIsAdminUsers"] = true
	ctx.Data["DefaultUserVisibilityMode"] = setting.Service.DefaultUserVisibilityMode
	ctx.Data["AllowedUserVisibilityModes"] = setting.Service.AllowedUserVisibilityModesSlice.ToVisibleTypeSlice()

	sources, err := db.Find[auth.Source](ctx, auth.FindSourcesOptions{
		IsActive: optional.Some(true),
	})
	if err != nil {
		ctx.ServerError("auth.Sources", err)
		return
	}
	ctx.Data["Sources"] = sources

	ctx.Data["CanSendEmail"] = setting.MailService != nil

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplUserNew)
		return
	}

	u := &user_model.User{
		Name:      form.UserName,
		Email:     form.Email,
		Passwd:    form.Password,
		LoginType: auth.Plain,
	}

	overwriteDefault := &user_model.CreateUserOverwriteOptions{
		IsActive:   optional.Some(true),
		Visibility: &form.Visibility,
	}

	if len(form.LoginType) > 0 {
		fields := strings.Split(form.LoginType, "-")
		if len(fields) == 2 {
			lType, _ := strconv.ParseInt(fields[0], 10, 0)
			u.LoginType = auth.Type(lType)
			u.LoginSource, _ = strconv.ParseInt(fields[1], 10, 64)
			u.LoginName = form.LoginName
		}
	}
	if u.LoginType == auth.NoType || u.LoginType == auth.Plain {
		if len(form.Password) < setting.MinPasswordLength {
			ctx.Data["Err_Password"] = true
			ctx.RenderWithErr(ctx.Tr("auth.password_too_short", setting.MinPasswordLength), tplUserNew, &form)
			return
		}
		if !password.IsComplexEnough(form.Password) {
			ctx.Data["Err_Password"] = true
			ctx.RenderWithErr(password.BuildComplexityError(ctx.Locale), tplUserNew, &form)
			return
		}
		if err := password.IsPwned(ctx, form.Password); err != nil {
			ctx.Data["Err_Password"] = true
			errMsg := ctx.Tr("auth.password_pwned")
			if password.IsErrIsPwnedRequest(err) {
				log.Error(err.Error())
				errMsg = ctx.Tr("auth.password_pwned_err")
			}
			ctx.RenderWithErr(errMsg, tplUserNew, &form)
			return
		}
		u.MustChangePassword = form.MustChangePassword
	}

	if err := user_model.AdminCreateUser(ctx, u, overwriteDefault); err != nil {
		switch {
		case user_model.IsErrUserAlreadyExist(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("form.username_been_taken"), tplUserNew, &form)
		case user_model.IsErrEmailAlreadyUsed(err):
			ctx.Data["Err_Email"] = true
			ctx.RenderWithErr(ctx.Tr("form.email_been_used"), tplUserNew, &form)
		case user_model.IsErrEmailInvalid(err), user_model.IsErrEmailCharIsNotSupported(err):
			ctx.Data["Err_Email"] = true
			ctx.RenderWithErr(ctx.Tr("form.email_invalid"), tplUserNew, &form)
		case db.IsErrNameReserved(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("user.form.name_reserved", err.(db.ErrNameReserved).Name), tplUserNew, &form)
		case db.IsErrNamePatternNotAllowed(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("user.form.name_pattern_not_allowed", err.(db.ErrNamePatternNotAllowed).Pattern), tplUserNew, &form)
		case db.IsErrNameCharsNotAllowed(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("user.form.name_chars_not_allowed", err.(db.ErrNameCharsNotAllowed).Name), tplUserNew, &form)
		default:
			ctx.ServerError("CreateUser", err)
		}
		return
	}

	if !user_model.IsEmailDomainAllowed(u.Email) {
		ctx.Flash.Warning(ctx.Tr("form.email_domain_is_not_allowed", u.Email))
	}

	log.Trace("Account created by admin (%s): %s", ctx.Doer.Name, u.Name)

	// Send email notification.
	if form.SendNotify {
		mailer.SendRegisterNotifyMail(u)
	}

	ctx.Flash.Success(ctx.Tr("admin.users.new_success", u.Name))
	ctx.Redirect(setting.AppSubURL + "/admin/users/" + strconv.FormatInt(u.ID, 10))
}

func prepareUserInfo(ctx *context.Context) *user_model.User {
	u, err := user_model.GetUserByID(ctx, ctx.ParamsInt64(":userid"))
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.Redirect(setting.AppSubURL + "/admin/users")
		} else {
			ctx.ServerError("GetUserByID", err)
		}
		return nil
	}
	ctx.Data["User"] = u

	if u.LoginSource > 0 {
		ctx.Data["LoginSource"], err = auth.GetSourceByID(ctx, u.LoginSource)
		if err != nil {
			ctx.ServerError("auth.GetSourceByID", err)
			return nil
		}
	} else {
		ctx.Data["LoginSource"] = &auth.Source{}
	}

	sources, err := db.Find[auth.Source](ctx, auth.FindSourcesOptions{})
	if err != nil {
		ctx.ServerError("auth.Sources", err)
		return nil
	}
	ctx.Data["Sources"] = sources

	hasTOTP, err := auth.HasTwoFactorByUID(ctx, u.ID)
	if err != nil {
		ctx.ServerError("auth.HasTwoFactorByUID", err)
		return nil
	}
	hasWebAuthn, err := auth.HasWebAuthnRegistrationsByUID(ctx, u.ID)
	if err != nil {
		ctx.ServerError("auth.HasWebAuthnRegistrationsByUID", err)
		return nil
	}
	ctx.Data["TwoFactorEnabled"] = hasTOTP || hasWebAuthn

	return u
}

func ViewUser(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.users.details")
	ctx.Data["PageIsAdminUsers"] = true
	ctx.Data["DisableRegularOrgCreation"] = setting.Admin.DisableRegularOrgCreation
	ctx.Data["DisableMigrations"] = setting.Repository.DisableMigrations
	ctx.Data["AllowedUserVisibilityModes"] = setting.Service.AllowedUserVisibilityModesSlice.ToVisibleTypeSlice()

	u := prepareUserInfo(ctx)
	if ctx.Written() {
		return
	}

	repos, count, err := repo_model.SearchRepository(ctx, &repo_model.SearchRepoOptions{
		ListOptions: db.ListOptionsAll,
		OwnerID:     u.ID,
		OrderBy:     db.SearchOrderByAlphabetically,
		Private:     true,
		Collaborate: optional.Some(false),
	})
	if err != nil {
		ctx.ServerError("SearchRepository", err)
		return
	}

	ctx.Data["Repos"] = repos
	ctx.Data["ReposTotal"] = int(count)

	emails, err := user_model.GetEmailAddresses(ctx, u.ID)
	if err != nil {
		ctx.ServerError("GetEmailAddresses", err)
		return
	}
	ctx.Data["Emails"] = emails
	ctx.Data["EmailsTotal"] = len(emails)

	orgs, err := db.Find[org_model.Organization](ctx, org_model.FindOrgOptions{
		ListOptions:    db.ListOptionsAll,
		UserID:         u.ID,
		IncludePrivate: true,
	})
	if err != nil {
		ctx.ServerError("FindOrgs", err)
		return
	}

	ctx.Data["Users"] = orgs // needed to be able to use explore/user_list template
	ctx.Data["OrgsTotal"] = len(orgs)

	ctx.HTML(http.StatusOK, tplUserView)
}

func editUserCommon(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.users.edit_account")
	ctx.Data["PageIsAdminUsers"] = true
	ctx.Data["DisableRegularOrgCreation"] = setting.Admin.DisableRegularOrgCreation
	ctx.Data["DisableMigrations"] = setting.Repository.DisableMigrations
	ctx.Data["AllowedUserVisibilityModes"] = setting.Service.AllowedUserVisibilityModesSlice.ToVisibleTypeSlice()
	ctx.Data["DisableGravatar"] = setting.Config().Picture.DisableGravatar.Value(ctx)
}

// EditUser show editing user page
func EditUser(ctx *context.Context) {
	editUserCommon(ctx)
	prepareUserInfo(ctx)
	if ctx.Written() {
		return
	}

	ctx.HTML(http.StatusOK, tplUserEdit)
}

// EditUserPost response for editing user
func EditUserPost(ctx *context.Context) {
	editUserCommon(ctx)
	u := prepareUserInfo(ctx)
	if ctx.Written() {
		return
	}

	form := web.GetForm(ctx).(*forms.AdminEditUserForm)
	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplUserEdit)
		return
	}

	if form.UserName != "" {
		if err := user_service.RenameUser(ctx, u, form.UserName); err != nil {
			switch {
			case user_model.IsErrUserIsNotLocal(err):
				ctx.Data["Err_UserName"] = true
				ctx.RenderWithErr(ctx.Tr("form.username_change_not_local_user"), tplUserEdit, &form)
			case user_model.IsErrUserAlreadyExist(err):
				ctx.Data["Err_UserName"] = true
				ctx.RenderWithErr(ctx.Tr("form.username_been_taken"), tplUserEdit, &form)
			case db.IsErrNameReserved(err):
				ctx.Data["Err_UserName"] = true
				ctx.RenderWithErr(ctx.Tr("user.form.name_reserved", form.UserName), tplUserEdit, &form)
			case db.IsErrNamePatternNotAllowed(err):
				ctx.Data["Err_UserName"] = true
				ctx.RenderWithErr(ctx.Tr("user.form.name_pattern_not_allowed", form.UserName), tplUserEdit, &form)
			case db.IsErrNameCharsNotAllowed(err):
				ctx.Data["Err_UserName"] = true
				ctx.RenderWithErr(ctx.Tr("user.form.name_chars_not_allowed", form.UserName), tplUserEdit, &form)
			default:
				ctx.ServerError("RenameUser", err)
			}
			return
		}
	}

	authOpts := &user_service.UpdateAuthOptions{
		Password:  optional.FromNonDefault(form.Password),
		LoginName: optional.Some(form.LoginName),
	}

	// skip self Prohibit Login
	if ctx.Doer.ID == u.ID {
		authOpts.ProhibitLogin = optional.Some(false)
	} else {
		authOpts.ProhibitLogin = optional.Some(form.ProhibitLogin)
	}

	fields := strings.Split(form.LoginType, "-")
	if len(fields) == 2 {
		authSource, _ := strconv.ParseInt(fields[1], 10, 64)

		authOpts.LoginSource = optional.Some(authSource)
	}

	if err := user_service.UpdateAuth(ctx, u, authOpts); err != nil {
		switch {
		case errors.Is(err, password.ErrMinLength):
			ctx.Data["Err_Password"] = true
			ctx.RenderWithErr(ctx.Tr("auth.password_too_short", setting.MinPasswordLength), tplUserEdit, &form)
		case errors.Is(err, password.ErrComplexity):
			ctx.Data["Err_Password"] = true
			ctx.RenderWithErr(password.BuildComplexityError(ctx.Locale), tplUserEdit, &form)
		case errors.Is(err, password.ErrIsPwned):
			ctx.Data["Err_Password"] = true
			ctx.RenderWithErr(ctx.Tr("auth.password_pwned"), tplUserEdit, &form)
		case password.IsErrIsPwnedRequest(err):
			ctx.Data["Err_Password"] = true
			ctx.RenderWithErr(ctx.Tr("auth.password_pwned_err"), tplUserEdit, &form)
		default:
			ctx.ServerError("UpdateUser", err)
		}
		return
	}

	if form.Email != "" {
		if err := user_service.AdminAddOrSetPrimaryEmailAddress(ctx, u, form.Email); err != nil {
			switch {
			case user_model.IsErrEmailCharIsNotSupported(err), user_model.IsErrEmailInvalid(err):
				ctx.Data["Err_Email"] = true
				ctx.RenderWithErr(ctx.Tr("form.email_invalid"), tplUserEdit, &form)
			case user_model.IsErrEmailAlreadyUsed(err):
				ctx.Data["Err_Email"] = true
				ctx.RenderWithErr(ctx.Tr("form.email_been_used"), tplUserEdit, &form)
			default:
				ctx.ServerError("AddOrSetPrimaryEmailAddress", err)
			}
			return
		}
		if !user_model.IsEmailDomainAllowed(form.Email) {
			ctx.Flash.Warning(ctx.Tr("form.email_domain_is_not_allowed", form.Email))
		}
	}

	opts := &user_service.UpdateOptions{
		FullName:                optional.Some(form.FullName),
		Website:                 optional.Some(form.Website),
		Location:                optional.Some(form.Location),
		IsActive:                optional.Some(form.Active),
		IsAdmin:                 optional.Some(form.Admin),
		AllowGitHook:            optional.Some(form.AllowGitHook),
		AllowImportLocal:        optional.Some(form.AllowImportLocal),
		MaxRepoCreation:         optional.Some(form.MaxRepoCreation),
		AllowCreateOrganization: optional.Some(form.AllowCreateOrganization),
		IsRestricted:            optional.Some(form.Restricted),
		Visibility:              optional.Some(form.Visibility),
		Language:                optional.Some(form.Language),
	}

	if err := user_service.UpdateUser(ctx, u, opts); err != nil {
		if models.IsErrDeleteLastAdminUser(err) {
			ctx.RenderWithErr(ctx.Tr("auth.last_admin"), tplUserEdit, &form)
		} else {
			ctx.ServerError("UpdateUser", err)
		}
		return
	}
	log.Trace("Account profile updated by admin (%s): %s", ctx.Doer.Name, u.Name)

	if form.Reset2FA {
		tf, err := auth.GetTwoFactorByUID(ctx, u.ID)
		if err != nil && !auth.IsErrTwoFactorNotEnrolled(err) {
			ctx.ServerError("auth.GetTwoFactorByUID", err)
			return
		} else if tf != nil {
			if err := auth.DeleteTwoFactorByID(ctx, tf.ID, u.ID); err != nil {
				ctx.ServerError("auth.DeleteTwoFactorByID", err)
				return
			}
		}

		wn, err := auth.GetWebAuthnCredentialsByUID(ctx, u.ID)
		if err != nil {
			ctx.ServerError("auth.GetTwoFactorByUID", err)
			return
		}
		for _, cred := range wn {
			if _, err := auth.DeleteCredential(ctx, cred.ID, u.ID); err != nil {
				ctx.ServerError("auth.DeleteCredential", err)
				return
			}
		}
	}

	ctx.Flash.Success(ctx.Tr("admin.users.update_profile_success"))
	ctx.Redirect(setting.AppSubURL + "/admin/users/" + url.PathEscape(ctx.Params(":userid")))
}

// DeleteUser response for deleting a user
func DeleteUser(ctx *context.Context) {
	u, err := user_model.GetUserByID(ctx, ctx.ParamsInt64(":userid"))
	if err != nil {
		ctx.ServerError("GetUserByID", err)
		return
	}

	// admin should not delete themself
	if u.ID == ctx.Doer.ID {
		ctx.Flash.Error(ctx.Tr("admin.users.cannot_delete_self"))
		ctx.Redirect(setting.AppSubURL + "/admin/users/" + url.PathEscape(ctx.Params(":userid")))
		return
	}

	if err = user_service.DeleteUser(ctx, u, ctx.FormBool("purge")); err != nil {
		switch {
		case models.IsErrUserOwnRepos(err):
			ctx.Flash.Error(ctx.Tr("admin.users.still_own_repo"))
			ctx.Redirect(setting.AppSubURL + "/admin/users/" + url.PathEscape(ctx.Params(":userid")))
		case models.IsErrUserHasOrgs(err):
			ctx.Flash.Error(ctx.Tr("admin.users.still_has_org"))
			ctx.Redirect(setting.AppSubURL + "/admin/users/" + url.PathEscape(ctx.Params(":userid")))
		case models.IsErrUserOwnPackages(err):
			ctx.Flash.Error(ctx.Tr("admin.users.still_own_packages"))
			ctx.Redirect(setting.AppSubURL + "/admin/users/" + url.PathEscape(ctx.Params(":userid")))
		case models.IsErrDeleteLastAdminUser(err):
			ctx.Flash.Error(ctx.Tr("auth.last_admin"))
			ctx.Redirect(setting.AppSubURL + "/admin/users/" + url.PathEscape(ctx.Params(":userid")))
		default:
			ctx.ServerError("DeleteUser", err)
		}
		return
	}
	log.Trace("Account deleted by admin (%s): %s", ctx.Doer.Name, u.Name)

	ctx.Flash.Success(ctx.Tr("admin.users.deletion_success"))
	ctx.Redirect(setting.AppSubURL + "/admin/users")
}

// AvatarPost response for change user's avatar request
func AvatarPost(ctx *context.Context) {
	u := prepareUserInfo(ctx)
	if ctx.Written() {
		return
	}

	form := web.GetForm(ctx).(*forms.AvatarForm)
	if err := user_setting.UpdateAvatarSetting(ctx, form, u); err != nil {
		ctx.Flash.Error(err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("settings.update_user_avatar_success"))
	}

	ctx.Redirect(setting.AppSubURL + "/admin/users/" + strconv.FormatInt(u.ID, 10))
}

// DeleteAvatar render delete avatar page
func DeleteAvatar(ctx *context.Context) {
	u := prepareUserInfo(ctx)
	if ctx.Written() {
		return
	}

	if err := user_service.DeleteAvatar(ctx, u); err != nil {
		ctx.Flash.Error(err.Error())
	}

	ctx.JSONRedirect(setting.AppSubURL + "/admin/users/" + strconv.FormatInt(u.ID, 10))
}
