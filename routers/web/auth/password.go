// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"errors"
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/auth/password"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/mailer"
	user_service "code.gitea.io/gitea/services/user"
)

var (
	// tplMustChangePassword template for updating a user's password
	tplMustChangePassword templates.TplName = "user/auth/change_passwd"
	tplForgotPassword     templates.TplName = "user/auth/forgot_passwd"
	tplResetPassword      templates.TplName = "user/auth/reset_passwd"
)

// ForgotPasswd render the forget password page
func ForgotPasswd(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("auth.forgot_password_title")

	if setting.MailService == nil {
		log.Warn("no mail service configured")
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
		ctx.NotFound(nil)
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

	if ctx.Cache.IsExist("MailResendLimit_" + u.LowerName) {
		ctx.Data["ResendLimited"] = true
		ctx.HTML(http.StatusOK, tplForgotPassword)
		return
	}

	mailer.SendResetPasswordMail(u)

	if err = ctx.Cache.Put("MailResendLimit_"+u.LowerName, u.LowerName, 180); err != nil {
		log.Error("Set cache(MailResendLimit) fail: %v", err)
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
		ctx.Flash.Error(ctx.Tr("auth.invalid_code_forgot_password", fmt.Sprintf("%s/user/forgot_password", setting.AppSubURL)), true)
		return nil, nil
	}

	// Fail early, don't frustrate the user
	u := user_model.VerifyUserTimeLimitCode(ctx, &user_model.TimeLimitCodeOptions{Purpose: user_model.TimeLimitCodeResetPassword}, code)
	if u == nil {
		ctx.Flash.Error(ctx.Tr("auth.invalid_code_forgot_password", fmt.Sprintf("%s/user/forgot_password", setting.AppSubURL)), true)
		return nil, nil
	}

	twofa, err := auth.GetTwoFactorByUID(ctx, u.ID)
	if err != nil {
		if !auth.IsErrTwoFactorNotEnrolled(err) {
			ctx.HTTPError(http.StatusInternalServerError, "CommonResetPassword", err.Error())
			return nil, nil
		}
	} else {
		ctx.Data["has_two_factor"] = true
		ctx.Data["scratch_code"] = ctx.FormBool("scratch_code")
	}

	// Show the user that they are affecting the account that they intended to
	ctx.Data["user_email"] = u.Email

	if nil != ctx.Doer && u.ID != ctx.Doer.ID {
		ctx.Flash.Error(ctx.Tr("auth.reset_password_wrong_user", ctx.Doer.Email, u.Email), true)
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
				ctx.HTTPError(http.StatusInternalServerError, "ValidateTOTP", err.Error())
				return
			}
			if !ok || twofa.LastUsedPasscode == passcode {
				ctx.Data["IsResetForm"] = true
				ctx.Data["Err_Passcode"] = true
				ctx.RenderWithErr(ctx.Tr("auth.twofa_passcode_incorrect"), tplResetPassword, nil)
				return
			}

			twofa.LastUsedPasscode = passcode
			if err = auth.UpdateTwoFactor(ctx, twofa); err != nil {
				ctx.ServerError("ResetPasswdPost: UpdateTwoFactor", err)
				return
			}
		}
	}

	opts := &user_service.UpdateAuthOptions{
		Password:           optional.Some(ctx.FormString("password")),
		MustChangePassword: optional.Some(false),
	}
	if err := user_service.UpdateAuth(ctx, u, opts); err != nil {
		ctx.Data["IsResetForm"] = true
		ctx.Data["Err_Password"] = true
		switch {
		case errors.Is(err, password.ErrMinLength):
			ctx.RenderWithErr(ctx.Tr("auth.password_too_short", setting.MinPasswordLength), tplResetPassword, nil)
		case errors.Is(err, password.ErrComplexity):
			ctx.RenderWithErr(password.BuildComplexityError(ctx.Locale), tplResetPassword, nil)
		case errors.Is(err, password.ErrIsPwned):
			ctx.RenderWithErr(ctx.Tr("auth.password_pwned", "https://haveibeenpwned.com/Passwords"), tplResetPassword, nil)
		case password.IsErrIsPwnedRequest(err):
			ctx.RenderWithErr(ctx.Tr("auth.password_pwned_err"), tplResetPassword, nil)
		default:
			ctx.ServerError("UpdateAuth", err)
		}
		return
	}

	log.Trace("User password reset: %s", u.Name)
	ctx.Data["IsResetFailed"] = true
	remember := len(ctx.FormString("remember")) != 0

	if regenerateScratchToken {
		// Invalidate the scratch token.
		_, err := twofa.GenerateScratchToken()
		if err != nil {
			ctx.ServerError("UserSignIn", err)
			return
		}
		if err = auth.UpdateTwoFactor(ctx, twofa); err != nil {
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

	// Make sure only requests for users who are eligible to change their password via
	// this method passes through
	if !ctx.Doer.MustChangePassword {
		ctx.ServerError("MustUpdatePassword", errors.New("cannot update password. Please visit the settings page"))
		return
	}

	if form.Password != form.Retype {
		ctx.Data["Err_Password"] = true
		ctx.RenderWithErr(ctx.Tr("form.password_not_match"), tplMustChangePassword, &form)
		return
	}

	opts := &user_service.UpdateAuthOptions{
		Password:           optional.Some(form.Password),
		MustChangePassword: optional.Some(false),
	}
	if err := user_service.UpdateAuth(ctx, ctx.Doer, opts); err != nil {
		switch {
		case errors.Is(err, password.ErrMinLength):
			ctx.Data["Err_Password"] = true
			ctx.RenderWithErr(ctx.Tr("auth.password_too_short", setting.MinPasswordLength), tplMustChangePassword, &form)
		case errors.Is(err, password.ErrComplexity):
			ctx.Data["Err_Password"] = true
			ctx.RenderWithErr(password.BuildComplexityError(ctx.Locale), tplMustChangePassword, &form)
		case errors.Is(err, password.ErrIsPwned):
			ctx.Data["Err_Password"] = true
			ctx.RenderWithErr(ctx.Tr("auth.password_pwned", "https://haveibeenpwned.com/Passwords"), tplMustChangePassword, &form)
		case password.IsErrIsPwnedRequest(err):
			ctx.Data["Err_Password"] = true
			ctx.RenderWithErr(ctx.Tr("auth.password_pwned_err"), tplMustChangePassword, &form)
		default:
			ctx.ServerError("UpdateAuth", err)
		}
		return
	}

	ctx.Flash.Success(ctx.Tr("settings.change_password_success"))

	log.Trace("User updated password: %s", ctx.Doer.Name)

	if redirectTo := ctx.GetSiteCookie("redirect_to"); redirectTo != "" {
		middleware.DeleteRedirectToCookie(ctx.Resp)
		ctx.RedirectToCurrentSite(redirectTo)
		return
	}

	ctx.Redirect(setting.AppSubURL + "/")
}
