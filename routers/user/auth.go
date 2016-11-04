// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"fmt"
	"net/url"

	"github.com/go-macaron/captcha"

	"github.com/go-gitea/gitea/models"
	"github.com/go-gitea/gitea/modules/auth"
	"github.com/go-gitea/gitea/modules/base"
	"github.com/go-gitea/gitea/modules/context"
	"github.com/go-gitea/gitea/modules/log"
	"github.com/go-gitea/gitea/modules/setting"
)

const (
	SIGNIN          base.TplName = "user/auth/signin"
	SIGNUP          base.TplName = "user/auth/signup"
	ACTIVATE        base.TplName = "user/auth/activate"
	FORGOT_PASSWORD base.TplName = "user/auth/forgot_passwd"
	RESET_PASSWORD  base.TplName = "user/auth/reset_passwd"
)

// AutoSignIn reads cookie and try to auto-login.
func AutoSignIn(ctx *context.Context) (bool, error) {
	if !models.HasEngine {
		return false, nil
	}

	uname := ctx.GetCookie(setting.CookieUserName)
	if len(uname) == 0 {
		return false, nil
	}

	isSucceed := false
	defer func() {
		if !isSucceed {
			log.Trace("auto-login cookie cleared: %s", uname)
			ctx.SetCookie(setting.CookieUserName, "", -1, setting.AppSubUrl)
			ctx.SetCookie(setting.CookieRememberName, "", -1, setting.AppSubUrl)
		}
	}()

	u, err := models.GetUserByName(uname)
	if err != nil {
		if !models.IsErrUserNotExist(err) {
			return false, fmt.Errorf("GetUserByName: %v", err)
		}
		return false, nil
	}

	if val, _ := ctx.GetSuperSecureCookie(
		base.EncodeMD5(u.Rands+u.Passwd), setting.CookieRememberName); val != u.Name {
		return false, nil
	}

	isSucceed = true
	ctx.Session.Set("uid", u.ID)
	ctx.Session.Set("uname", u.Name)
	ctx.SetCookie(setting.CSRFCookieName, "", -1, setting.AppSubUrl)
	return true, nil
}

func SignIn(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("sign_in")

	// Check auto-login.
	isSucceed, err := AutoSignIn(ctx)
	if err != nil {
		ctx.Handle(500, "AutoSignIn", err)
		return
	}

	redirectTo := ctx.Query("redirect_to")
	if len(redirectTo) > 0 {
		ctx.SetCookie("redirect_to", redirectTo, 0, setting.AppSubUrl)
	} else {
		redirectTo, _ = url.QueryUnescape(ctx.GetCookie("redirect_to"))
	}

	if isSucceed {
		if len(redirectTo) > 0 {
			ctx.SetCookie("redirect_to", "", -1, setting.AppSubUrl)
			ctx.Redirect(redirectTo)
		} else {
			ctx.Redirect(setting.AppSubUrl + "/")
		}
		return
	}

	ctx.HTML(200, SIGNIN)
}

func SignInPost(ctx *context.Context, form auth.SignInForm) {
	ctx.Data["Title"] = ctx.Tr("sign_in")

	if ctx.HasError() {
		ctx.HTML(200, SIGNIN)
		return
	}

	u, err := models.UserSignIn(form.UserName, form.Password)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.RenderWithErr(ctx.Tr("form.username_password_incorrect"), SIGNIN, &form)
		} else {
			ctx.Handle(500, "UserSignIn", err)
		}
		return
	}

	if form.Remember {
		days := 86400 * setting.LogInRememberDays
		ctx.SetCookie(setting.CookieUserName, u.Name, days, setting.AppSubUrl)
		ctx.SetSuperSecureCookie(base.EncodeMD5(u.Rands+u.Passwd),
			setting.CookieRememberName, u.Name, days, setting.AppSubUrl)
	}

	ctx.Session.Set("uid", u.ID)
	ctx.Session.Set("uname", u.Name)

	// Clear whatever CSRF has right now, force to generate a new one
	ctx.SetCookie(setting.CSRFCookieName, "", -1, setting.AppSubUrl)

	if redirectTo, _ := url.QueryUnescape(ctx.GetCookie("redirect_to")); len(redirectTo) > 0 {
		ctx.SetCookie("redirect_to", "", -1, setting.AppSubUrl)
		ctx.Redirect(redirectTo)
		return
	}

	ctx.Redirect(setting.AppSubUrl + "/")
}

func SignOut(ctx *context.Context) {
	ctx.Session.Delete("uid")
	ctx.Session.Delete("uname")
	ctx.Session.Delete("socialId")
	ctx.Session.Delete("socialName")
	ctx.Session.Delete("socialEmail")
	ctx.SetCookie(setting.CookieUserName, "", -1, setting.AppSubUrl)
	ctx.SetCookie(setting.CookieRememberName, "", -1, setting.AppSubUrl)
	ctx.SetCookie(setting.CSRFCookieName, "", -1, setting.AppSubUrl)
	ctx.Redirect(setting.AppSubUrl + "/")
}

func SignUp(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("sign_up")

	ctx.Data["EnableCaptcha"] = setting.Service.EnableCaptcha

	if setting.Service.DisableRegistration {
		ctx.Data["DisableRegistration"] = true
		ctx.HTML(200, SIGNUP)
		return
	}

	ctx.HTML(200, SIGNUP)
}

func SignUpPost(ctx *context.Context, cpt *captcha.Captcha, form auth.RegisterForm) {
	ctx.Data["Title"] = ctx.Tr("sign_up")

	ctx.Data["EnableCaptcha"] = setting.Service.EnableCaptcha

	if setting.Service.DisableRegistration {
		ctx.Error(403)
		return
	}

	if ctx.HasError() {
		ctx.HTML(200, SIGNUP)
		return
	}

	if setting.Service.EnableCaptcha && !cpt.VerifyReq(ctx.Req) {
		ctx.Data["Err_Captcha"] = true
		ctx.RenderWithErr(ctx.Tr("form.captcha_incorrect"), SIGNUP, &form)
		return
	}

	if form.Password != form.Retype {
		ctx.Data["Err_Password"] = true
		ctx.RenderWithErr(ctx.Tr("form.password_not_match"), SIGNUP, &form)
		return
	}

	u := &models.User{
		Name:     form.UserName,
		Email:    form.Email,
		Passwd:   form.Password,
		IsActive: !setting.Service.RegisterEmailConfirm,
	}
	if err := models.CreateUser(u); err != nil {
		switch {
		case models.IsErrUserAlreadyExist(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("form.username_been_taken"), SIGNUP, &form)
		case models.IsErrEmailAlreadyUsed(err):
			ctx.Data["Err_Email"] = true
			ctx.RenderWithErr(ctx.Tr("form.email_been_used"), SIGNUP, &form)
		case models.IsErrNameReserved(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("user.form.name_reserved", err.(models.ErrNameReserved).Name), SIGNUP, &form)
		case models.IsErrNamePatternNotAllowed(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("user.form.name_pattern_not_allowed", err.(models.ErrNamePatternNotAllowed).Pattern), SIGNUP, &form)
		default:
			ctx.Handle(500, "CreateUser", err)
		}
		return
	}
	log.Trace("Account created: %s", u.Name)

	// Auto-set admin for the only user.
	if models.CountUsers() == 1 {
		u.IsAdmin = true
		u.IsActive = true
		if err := models.UpdateUser(u); err != nil {
			ctx.Handle(500, "UpdateUser", err)
			return
		}
	}

	// Send confirmation email, no need for social account.
	if setting.Service.RegisterEmailConfirm && u.ID > 1 {
		models.SendActivateAccountMail(ctx.Context, u)
		ctx.Data["IsSendRegisterMail"] = true
		ctx.Data["Email"] = u.Email
		ctx.Data["Hours"] = setting.Service.ActiveCodeLives / 60
		ctx.HTML(200, ACTIVATE)

		if err := ctx.Cache.Put("MailResendLimit_"+u.LowerName, u.LowerName, 180); err != nil {
			log.Error(4, "Set cache(MailResendLimit) fail: %v", err)
		}
		return
	}

	ctx.Redirect(setting.AppSubUrl + "/user/login")
}

func Activate(ctx *context.Context) {
	code := ctx.Query("code")
	if len(code) == 0 {
		ctx.Data["IsActivatePage"] = true
		if ctx.User.IsActive {
			ctx.Error(404)
			return
		}
		// Resend confirmation email.
		if setting.Service.RegisterEmailConfirm {
			if ctx.Cache.IsExist("MailResendLimit_" + ctx.User.LowerName) {
				ctx.Data["ResendLimited"] = true
			} else {
				ctx.Data["Hours"] = setting.Service.ActiveCodeLives / 60
				models.SendActivateAccountMail(ctx.Context, ctx.User)

				if err := ctx.Cache.Put("MailResendLimit_"+ctx.User.LowerName, ctx.User.LowerName, 180); err != nil {
					log.Error(4, "Set cache(MailResendLimit) fail: %v", err)
				}
			}
		} else {
			ctx.Data["ServiceNotEnabled"] = true
		}
		ctx.HTML(200, ACTIVATE)
		return
	}

	// Verify code.
	if user := models.VerifyUserActiveCode(code); user != nil {
		user.IsActive = true
		user.Rands = models.GetUserSalt()
		if err := models.UpdateUser(user); err != nil {
			if models.IsErrUserNotExist(err) {
				ctx.Error(404)
			} else {
				ctx.Handle(500, "UpdateUser", err)
			}
			return
		}

		log.Trace("User activated: %s", user.Name)

		ctx.Session.Set("uid", user.ID)
		ctx.Session.Set("uname", user.Name)
		ctx.Redirect(setting.AppSubUrl + "/")
		return
	}

	ctx.Data["IsActivateFailed"] = true
	ctx.HTML(200, ACTIVATE)
}

func ActivateEmail(ctx *context.Context) {
	code := ctx.Query("code")
	email_string := ctx.Query("email")

	// Verify code.
	if email := models.VerifyActiveEmailCode(code, email_string); email != nil {
		if err := email.Activate(); err != nil {
			ctx.Handle(500, "ActivateEmail", err)
		}

		log.Trace("Email activated: %s", email.Email)
		ctx.Flash.Success(ctx.Tr("settings.add_email_success"))
	}

	ctx.Redirect(setting.AppSubUrl + "/user/settings/email")
	return
}

func ForgotPasswd(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("auth.forgot_password")

	if setting.MailService == nil {
		ctx.Data["IsResetDisable"] = true
		ctx.HTML(200, FORGOT_PASSWORD)
		return
	}

	ctx.Data["IsResetRequest"] = true
	ctx.HTML(200, FORGOT_PASSWORD)
}

func ForgotPasswdPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("auth.forgot_password")

	if setting.MailService == nil {
		ctx.Handle(403, "ForgotPasswdPost", nil)
		return
	}
	ctx.Data["IsResetRequest"] = true

	email := ctx.Query("email")
	ctx.Data["Email"] = email

	u, err := models.GetUserByEmail(email)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Data["Hours"] = setting.Service.ActiveCodeLives / 60
			ctx.Data["IsResetSent"] = true
			ctx.HTML(200, FORGOT_PASSWORD)
			return
		} else {
			ctx.Handle(500, "user.ResetPasswd(check existence)", err)
		}
		return
	}

	if !u.IsLocal() {
		ctx.Data["Err_Email"] = true
		ctx.RenderWithErr(ctx.Tr("auth.non_local_account"), FORGOT_PASSWORD, nil)
		return
	}

	if ctx.Cache.IsExist("MailResendLimit_" + u.LowerName) {
		ctx.Data["ResendLimited"] = true
		ctx.HTML(200, FORGOT_PASSWORD)
		return
	}

	models.SendResetPasswordMail(ctx.Context, u)
	if err = ctx.Cache.Put("MailResendLimit_"+u.LowerName, u.LowerName, 180); err != nil {
		log.Error(4, "Set cache(MailResendLimit) fail: %v", err)
	}

	ctx.Data["Hours"] = setting.Service.ActiveCodeLives / 60
	ctx.Data["IsResetSent"] = true
	ctx.HTML(200, FORGOT_PASSWORD)
}

func ResetPasswd(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("auth.reset_password")

	code := ctx.Query("code")
	if len(code) == 0 {
		ctx.Error(404)
		return
	}
	ctx.Data["Code"] = code
	ctx.Data["IsResetForm"] = true
	ctx.HTML(200, RESET_PASSWORD)
}

func ResetPasswdPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("auth.reset_password")

	code := ctx.Query("code")
	if len(code) == 0 {
		ctx.Error(404)
		return
	}
	ctx.Data["Code"] = code

	if u := models.VerifyUserActiveCode(code); u != nil {
		// Validate password length.
		passwd := ctx.Query("password")
		if len(passwd) < 6 {
			ctx.Data["IsResetForm"] = true
			ctx.Data["Err_Password"] = true
			ctx.RenderWithErr(ctx.Tr("auth.password_too_short"), RESET_PASSWORD, nil)
			return
		}

		u.Passwd = passwd
		u.Rands = models.GetUserSalt()
		u.Salt = models.GetUserSalt()
		u.EncodePasswd()
		if err := models.UpdateUser(u); err != nil {
			ctx.Handle(500, "UpdateUser", err)
			return
		}

		log.Trace("User password reset: %s", u.Name)
		ctx.Redirect(setting.AppSubUrl + "/user/login")
		return
	}

	ctx.Data["IsResetFailed"] = true
	ctx.HTML(200, RESET_PASSWORD)
}
