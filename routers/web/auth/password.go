// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"errors"
	"net/http"

	"code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/auth/password"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/routers/utils"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/mailer"
)

var (
	// tplMustChangePassword template for updating a user's password
	tplMustChangePassword base.TplName = "user/auth/change_passwd"
	tplForgotPassword     base.TplName = "user/auth/forgot_passwd"
	tplResetPassword      base.TplName = "user/auth/reset_passwd"
)

// ForgotPasswd render the forget password page
func ForgotPasswd(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("auth.forgot_password_title")

	if setting.MailService == nil {
		log.Warn(ctx.Tr("auth.disable_forgot_password_mail_admin"))
		ctx.Data["IsResetDisable"] = true
		ctx.HTML(http.StatusOK, tplForgotPassword)
		return
	}

	ctx.Data["Email"] = ctx.FormString("email")

	ctx.Data["IsResetRequest"] = true
	ctx.HTML(http.StatusOK, tplForgotPassword)
}

// ForgotPasswdPost response for forget password request
func ForgotPasswdPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("auth.forgot_password_title")

	if setting.MailService == nil {
		ctx.NotFound("ForgotPasswdPost", nil)
		return
	}
	ctx.Data["IsResetRequest"] = true

	email := ctx.FormString("email")
	ctx.Data["Email"] = email

	u, err := user_model.GetUserByEmail(ctx, email)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.Data["ResetPwdCodeLives"] = timeutil.MinutesToFriendly(setting.Service.ResetPwdCodeLives, ctx.Locale)
			ctx.Data["IsResetSent"] = true
			ctx.HTML(http.StatusOK, tplForgotPassword)
			return
		}

		ctx.ServerError("user.ResetPasswd(check existence)", err)
		return
	}

	if !u.IsLocal() && !u.IsOAuth2() {
		ctx.Data["Err_Email"] = true
		ctx.RenderWithErr(ctx.Tr("auth.non_local_account"), tplForgotPassword, nil)
		return
	}

	if setting.CacheService.Enabled && ctx.Cache.IsExist("MailResendLimit_"+u.LowerName) {
		ctx.Data["ResendLimited"] = true
		ctx.HTML(http.StatusOK, tplForgotPassword)
		return
	}

	mailer.SendResetPasswordMail(u)

	if setting.CacheService.Enabled {
		if err = ctx.Cache.Put("MailResendLimit_"+u.LowerName, u.LowerName, 180); err != nil {
			log.Error("Set cache(MailResendLimit) fail: %v", err)
		}
	}

	ctx.Data["ResetPwdCodeLives"] = timeutil.MinutesToFriendly(setting.Service.ResetPwdCodeLives, ctx.Locale)
	ctx.Data["IsResetSent"] = true
	ctx.HTML(http.StatusOK, tplForgotPassword)
}

func commonResetPassword(ctx *context.Context) (*user_model.User, *auth.TwoFactor) {
	code := ctx.FormString("code")

	ctx.Data["Title"] = ctx.Tr("auth.reset_password")
	ctx.Data["Code"] = code

	if nil != ctx.Doer {
		ctx.Data["user_signed_in"] = true
	}

	if len(code) == 0 {
		ctx.Flash.Error(ctx.Tr("auth.invalid_code"))
		return nil, nil
	}

	// Fail early, don't frustrate the user
	u := user_model.VerifyUserActiveCode(code)
	if u == nil {
		ctx.Flash.Error(ctx.Tr("auth.invalid_code"))
		return nil, nil
	}

	twofa, err := auth.GetTwoFactorByUID(u.ID)
	if err != nil {
		if !auth.IsErrTwoFactorNotEnrolled(err) {
			ctx.Error(http.StatusInternalServerError, "CommonResetPassword", err.Error())
			return nil, nil
		}
	} else {
		ctx.Data["has_two_factor"] = true
		ctx.Data["scratch_code"] = ctx.FormBool("scratch_code")
	}

	// Show the user that they are affecting the account that they intended to
	ctx.Data["user_email"] = u.Email

	if nil != ctx.Doer && u.ID != ctx.Doer.ID {
		ctx.Flash.Error(ctx.Tr("auth.reset_password_wrong_user", ctx.Doer.Email, u.Email))
		return nil, nil
	}

	return u, twofa
}

// ResetPasswd render the account recovery page
func ResetPasswd(ctx *context.Context) {
	ctx.Data["IsResetForm"] = true

	commonResetPassword(ctx)
	if ctx.Written() {
		return
	}

	ctx.HTML(http.StatusOK, tplResetPassword)
}

// ResetPasswdPost response from account recovery request
func ResetPasswdPost(ctx *context.Context) {
	u, twofa := commonResetPassword(ctx)
	if ctx.Written() {
		return
	}

	if u == nil {
		// Flash error has been set
		ctx.HTML(http.StatusOK, tplResetPassword)
		return
	}

	// Validate password length.
	passwd := ctx.FormString("password")
	if len(passwd) < setting.MinPasswordLength {
		ctx.Data["IsResetForm"] = true
		ctx.Data["Err_Password"] = true
		ctx.RenderWithErr(ctx.Tr("auth.password_too_short", setting.MinPasswordLength), tplResetPassword, nil)
		return
	} else if !password.IsComplexEnough(passwd) {
		ctx.Data["IsResetForm"] = true
		ctx.Data["Err_Password"] = true
		ctx.RenderWithErr(password.BuildComplexityError(ctx), tplResetPassword, nil)
		return
	} else if pwned, err := password.IsPwned(ctx, passwd); pwned || err != nil {
		errMsg := ctx.Tr("auth.password_pwned")
		if err != nil {
			log.Error(err.Error())
			errMsg = ctx.Tr("auth.password_pwned_err")
		}
		ctx.Data["IsResetForm"] = true
		ctx.Data["Err_Password"] = true
		ctx.RenderWithErr(errMsg, tplResetPassword, nil)
		return
	}

	// Handle two-factor
	regenerateScratchToken := false
	if twofa != nil {
		if ctx.FormBool("scratch_code") {
			if !twofa.VerifyScratchToken(ctx.FormString("token")) {
				ctx.Data["IsResetForm"] = true
				ctx.Data["Err_Token"] = true
				ctx.RenderWithErr(ctx.Tr("auth.twofa_scratch_token_incorrect"), tplResetPassword, nil)
				return
			}
			regenerateScratchToken = true
		} else {
			passcode := ctx.FormString("passcode")
			ok, err := twofa.ValidateTOTP(passcode)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "ValidateTOTP", err.Error())
				return
			}
			if !ok || twofa.LastUsedPasscode == passcode {
				ctx.Data["IsResetForm"] = true
				ctx.Data["Err_Passcode"] = true
				ctx.RenderWithErr(ctx.Tr("auth.twofa_passcode_incorrect"), tplResetPassword, nil)
				return
			}

			twofa.LastUsedPasscode = passcode
			if err = auth.UpdateTwoFactor(twofa); err != nil {
				ctx.ServerError("ResetPasswdPost: UpdateTwoFactor", err)
				return
			}
		}
	}
	var err error
	if u.Rands, err = user_model.GetUserSalt(); err != nil {
		ctx.ServerError("UpdateUser", err)
		return
	}
	if err = u.SetPassword(passwd); err != nil {
		ctx.ServerError("UpdateUser", err)
		return
	}
	u.MustChangePassword = false
	if err := user_model.UpdateUserCols(ctx, u, "must_change_password", "passwd", "passwd_hash_algo", "rands", "salt"); err != nil {
		ctx.ServerError("UpdateUser", err)
		return
	}

	log.Trace("User password reset: %s", u.Name)
	ctx.Data["IsResetFailed"] = true
	remember := len(ctx.FormString("remember")) != 0

	if regenerateScratchToken {
		// Invalidate the scratch token.
		_, err = twofa.GenerateScratchToken()
		if err != nil {
			ctx.ServerError("UserSignIn", err)
			return
		}
		if err = auth.UpdateTwoFactor(twofa); err != nil {
			ctx.ServerError("UserSignIn", err)
			return
		}

		handleSignInFull(ctx, u, remember, false)
		if ctx.Written() {
			return
		}
		ctx.Flash.Info(ctx.Tr("auth.twofa_scratch_used"))
		ctx.Redirect(setting.AppSubURL + "/user/settings/security")
		return
	}

	handleSignIn(ctx, u, remember)
}

// MustChangePassword renders the page to change a user's password
func MustChangePassword(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("auth.must_change_password")
	ctx.Data["ChangePasscodeLink"] = setting.AppSubURL + "/user/settings/change_password"
	ctx.Data["MustChangePassword"] = true
	ctx.HTML(http.StatusOK, tplMustChangePassword)
}

// MustChangePasswordPost response for updating a user's password after their
// account was created by an admin
func MustChangePasswordPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.MustChangePasswordForm)
	ctx.Data["Title"] = ctx.Tr("auth.must_change_password")
	ctx.Data["ChangePasscodeLink"] = setting.AppSubURL + "/user/settings/change_password"
	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplMustChangePassword)
		return
	}
	u := ctx.Doer
	// Make sure only requests for users who are eligible to change their password via
	// this method passes through
	if !u.MustChangePassword {
		ctx.ServerError("MustUpdatePassword", errors.New("cannot update password.. Please visit the settings page"))
		return
	}

	if form.Password != form.Retype {
		ctx.Data["Err_Password"] = true
		ctx.RenderWithErr(ctx.Tr("form.password_not_match"), tplMustChangePassword, &form)
		return
	}

	if len(form.Password) < setting.MinPasswordLength {
		ctx.Data["Err_Password"] = true
		ctx.RenderWithErr(ctx.Tr("auth.password_too_short", setting.MinPasswordLength), tplMustChangePassword, &form)
		return
	}

	if !password.IsComplexEnough(form.Password) {
		ctx.Data["Err_Password"] = true
		ctx.RenderWithErr(password.BuildComplexityError(ctx), tplMustChangePassword, &form)
		return
	}
	pwned, err := password.IsPwned(ctx, form.Password)
	if pwned {
		ctx.Data["Err_Password"] = true
		errMsg := ctx.Tr("auth.password_pwned")
		if err != nil {
			log.Error(err.Error())
			errMsg = ctx.Tr("auth.password_pwned_err")
		}
		ctx.RenderWithErr(errMsg, tplMustChangePassword, &form)
		return
	}

	if err = u.SetPassword(form.Password); err != nil {
		ctx.ServerError("UpdateUser", err)
		return
	}

	u.MustChangePassword = false

	if err := user_model.UpdateUserCols(ctx, u, "must_change_password", "passwd", "passwd_hash_algo", "salt"); err != nil {
		ctx.ServerError("UpdateUser", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("settings.change_password_success"))

	log.Trace("User updated password: %s", u.Name)

	if redirectTo := ctx.GetSiteCookie("redirect_to"); len(redirectTo) > 0 && !utils.IsExternalURL(redirectTo) {
		middleware.DeleteRedirectToCookie(ctx.Resp)
		ctx.RedirectToFirst(redirectTo)
		return
	}

	ctx.Redirect(setting.AppSubURL + "/")
}
