// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/eventsource"
	"code.gitea.io/gitea/modules/hcaptcha"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/password"
	"code.gitea.io/gitea/modules/recaptcha"
	"code.gitea.io/gitea/modules/session"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/routers/utils"
	auth_service "code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/auth/source/oauth2"
	"code.gitea.io/gitea/services/externalaccount"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/mailer"

	"github.com/markbates/goth"
)

const (
	// tplSignIn template for sign in page
	tplSignIn base.TplName = "user/auth/signin"
	// tplSignUp template path for sign up page
	tplSignUp base.TplName = "user/auth/signup"
	// TplActivate template path for activate user
	TplActivate base.TplName = "user/auth/activate"
)

// AutoSignIn reads cookie and try to auto-login.
func AutoSignIn(ctx *context.Context) (bool, error) {
	if !db.HasEngine {
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
			ctx.DeleteCookie(setting.CookieUserName)
			ctx.DeleteCookie(setting.CookieRememberName)
		}
	}()

	u, err := user_model.GetUserByName(uname)
	if err != nil {
		if !user_model.IsErrUserNotExist(err) {
			return false, fmt.Errorf("GetUserByName: %v", err)
		}
		return false, nil
	}

	if val, ok := ctx.GetSuperSecureCookie(
		base.EncodeMD5(u.Rands+u.Passwd), setting.CookieRememberName); !ok || val != u.Name {
		return false, nil
	}

	isSucceed = true

	if _, err := session.RegenerateSession(ctx.Resp, ctx.Req); err != nil {
		return false, fmt.Errorf("unable to RegenerateSession: Error: %w", err)
	}

	// Set session IDs
	if err := ctx.Session.Set("uid", u.ID); err != nil {
		return false, err
	}
	if err := ctx.Session.Set("uname", u.Name); err != nil {
		return false, err
	}
	if err := ctx.Session.Release(); err != nil {
		return false, err
	}

	if err := resetLocale(ctx, u); err != nil {
		return false, err
	}

	middleware.DeleteCSRFCookie(ctx.Resp)
	return true, nil
}

func resetLocale(ctx *context.Context, u *user_model.User) error {
	// Language setting of the user overwrites the one previously set
	// If the user does not have a locale set, we save the current one.
	if len(u.Language) == 0 {
		u.Language = ctx.Locale.Language()
		if err := user_model.UpdateUserCols(db.DefaultContext, u, "language"); err != nil {
			return err
		}
	}

	middleware.SetLocaleCookie(ctx.Resp, u.Language, 0)

	if ctx.Locale.Language() != u.Language {
		ctx.Locale = middleware.Locale(ctx.Resp, ctx.Req)
	}

	return nil
}

func checkAutoLogin(ctx *context.Context) bool {
	// Check auto-login
	isSucceed, err := AutoSignIn(ctx)
	if err != nil {
		ctx.ServerError("AutoSignIn", err)
		return true
	}

	redirectTo := ctx.FormString("redirect_to")
	if len(redirectTo) > 0 {
		middleware.SetRedirectToCookie(ctx.Resp, redirectTo)
	} else {
		redirectTo = ctx.GetCookie("redirect_to")
	}

	if isSucceed {
		middleware.DeleteRedirectToCookie(ctx.Resp)
		ctx.RedirectToFirst(redirectTo, setting.AppSubURL+string(setting.LandingPageURL))
		return true
	}

	return false
}

// SignIn render sign in page
func SignIn(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("sign_in")

	// Check auto-login
	if checkAutoLogin(ctx) {
		return
	}

	orderedOAuth2Names, oauth2Providers, err := oauth2.GetActiveOAuth2Providers()
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}
	ctx.Data["OrderedOAuth2Names"] = orderedOAuth2Names
	ctx.Data["OAuth2Providers"] = oauth2Providers
	ctx.Data["Title"] = ctx.Tr("sign_in")
	ctx.Data["SignInLink"] = setting.AppSubURL + "/user/login"
	ctx.Data["PageIsSignIn"] = true
	ctx.Data["PageIsLogin"] = true
	ctx.Data["EnableSSPI"] = auth.IsSSPIEnabled()

	ctx.HTML(http.StatusOK, tplSignIn)
}

// SignInPost response for sign in request
func SignInPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("sign_in")

	orderedOAuth2Names, oauth2Providers, err := oauth2.GetActiveOAuth2Providers()
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}
	ctx.Data["OrderedOAuth2Names"] = orderedOAuth2Names
	ctx.Data["OAuth2Providers"] = oauth2Providers
	ctx.Data["Title"] = ctx.Tr("sign_in")
	ctx.Data["SignInLink"] = setting.AppSubURL + "/user/login"
	ctx.Data["PageIsSignIn"] = true
	ctx.Data["PageIsLogin"] = true
	ctx.Data["EnableSSPI"] = auth.IsSSPIEnabled()

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplSignIn)
		return
	}

	form := web.GetForm(ctx).(*forms.SignInForm)
	u, source, err := auth_service.UserSignIn(form.UserName, form.Password)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.RenderWithErr(ctx.Tr("form.username_password_incorrect"), tplSignIn, &form)
			log.Info("Failed authentication attempt for %s from %s: %v", form.UserName, ctx.RemoteAddr(), err)
		} else if user_model.IsErrEmailAlreadyUsed(err) {
			ctx.RenderWithErr(ctx.Tr("form.email_been_used"), tplSignIn, &form)
			log.Info("Failed authentication attempt for %s from %s: %v", form.UserName, ctx.RemoteAddr(), err)
		} else if user_model.IsErrUserProhibitLogin(err) {
			log.Info("Failed authentication attempt for %s from %s: %v", form.UserName, ctx.RemoteAddr(), err)
			ctx.Data["Title"] = ctx.Tr("auth.prohibit_login")
			ctx.HTML(http.StatusOK, "user/auth/prohibit_login")
		} else if user_model.IsErrUserInactive(err) {
			if setting.Service.RegisterEmailConfirm {
				ctx.Data["Title"] = ctx.Tr("auth.active_your_account")
				ctx.HTML(http.StatusOK, TplActivate)
			} else {
				log.Info("Failed authentication attempt for %s from %s: %v", form.UserName, ctx.RemoteAddr(), err)
				ctx.Data["Title"] = ctx.Tr("auth.prohibit_login")
				ctx.HTML(http.StatusOK, "user/auth/prohibit_login")
			}
		} else {
			ctx.ServerError("UserSignIn", err)
		}
		return
	}

	// Now handle 2FA:

	// First of all if the source can skip local two fa we're done
	if skipper, ok := source.Cfg.(auth_service.LocalTwoFASkipper); ok && skipper.IsSkipLocalTwoFA() {
		handleSignIn(ctx, u, form.Remember)
		return
	}

	// If this user is enrolled in 2FA TOTP, we can't sign the user in just yet.
	// Instead, redirect them to the 2FA authentication page.
	hasTOTPtwofa, err := auth.HasTwoFactorByUID(u.ID)
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}

	// Check if the user has webauthn registration
	hasWebAuthnTwofa, err := auth.HasWebAuthnRegistrationsByUID(u.ID)
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}

	if !hasTOTPtwofa && !hasWebAuthnTwofa {
		// No two factor auth configured we can sign in the user
		handleSignIn(ctx, u, form.Remember)
		return
	}

	if _, err := session.RegenerateSession(ctx.Resp, ctx.Req); err != nil {
		ctx.ServerError("UserSignIn: Unable to set regenerate session", err)
		return
	}

	// User will need to use 2FA TOTP or WebAuthn, save data
	if err := ctx.Session.Set("twofaUid", u.ID); err != nil {
		ctx.ServerError("UserSignIn: Unable to set twofaUid in session", err)
		return
	}

	if err := ctx.Session.Set("twofaRemember", form.Remember); err != nil {
		ctx.ServerError("UserSignIn: Unable to set twofaRemember in session", err)
		return
	}

	if hasTOTPtwofa {
		// User will need to use U2F, save data
		if err := ctx.Session.Set("totpEnrolled", u.ID); err != nil {
			ctx.ServerError("UserSignIn: Unable to set WebAuthn Enrolled in session", err)
			return
		}
	}

	if err := ctx.Session.Release(); err != nil {
		ctx.ServerError("UserSignIn: Unable to save session", err)
		return
	}

	// If we have U2F redirect there first
	if hasWebAuthnTwofa {
		ctx.Redirect(setting.AppSubURL + "/user/webauthn")
		return
	}

	// Fallback to 2FA
	ctx.Redirect(setting.AppSubURL + "/user/two_factor")
}

// This handles the final part of the sign-in process of the user.
func handleSignIn(ctx *context.Context, u *user_model.User, remember bool) {
	redirect := handleSignInFull(ctx, u, remember, true)
	if ctx.Written() {
		return
	}
	ctx.Redirect(redirect)
}

func handleSignInFull(ctx *context.Context, u *user_model.User, remember, obeyRedirect bool) string {
	if remember {
		days := 86400 * setting.LogInRememberDays
		ctx.SetCookie(setting.CookieUserName, u.Name, days)
		ctx.SetSuperSecureCookie(base.EncodeMD5(u.Rands+u.Passwd),
			setting.CookieRememberName, u.Name, days)
	}

	if _, err := session.RegenerateSession(ctx.Resp, ctx.Req); err != nil {
		ctx.ServerError("RegenerateSession", err)
		return setting.AppSubURL + "/"
	}

	// Delete the openid, 2fa and linkaccount data
	_ = ctx.Session.Delete("openid_verified_uri")
	_ = ctx.Session.Delete("openid_signin_remember")
	_ = ctx.Session.Delete("openid_determined_email")
	_ = ctx.Session.Delete("openid_determined_username")
	_ = ctx.Session.Delete("twofaUid")
	_ = ctx.Session.Delete("twofaRemember")
	_ = ctx.Session.Delete("u2fChallenge")
	_ = ctx.Session.Delete("linkAccount")
	if err := ctx.Session.Set("uid", u.ID); err != nil {
		log.Error("Error setting uid %d in session: %v", u.ID, err)
	}
	if err := ctx.Session.Set("uname", u.Name); err != nil {
		log.Error("Error setting uname %s session: %v", u.Name, err)
	}
	if err := ctx.Session.Release(); err != nil {
		log.Error("Unable to store session: %v", err)
	}

	// Language setting of the user overwrites the one previously set
	// If the user does not have a locale set, we save the current one.
	if len(u.Language) == 0 {
		u.Language = ctx.Locale.Language()
		if err := user_model.UpdateUserCols(db.DefaultContext, u, "language"); err != nil {
			ctx.ServerError("UpdateUserCols Language", fmt.Errorf("Error updating user language [user: %d, locale: %s]", u.ID, u.Language))
			return setting.AppSubURL + "/"
		}
	}

	middleware.SetLocaleCookie(ctx.Resp, u.Language, 0)

	if ctx.Locale.Language() != u.Language {
		ctx.Locale = middleware.Locale(ctx.Resp, ctx.Req)
	}

	// Clear whatever CSRF has right now, force to generate a new one
	middleware.DeleteCSRFCookie(ctx.Resp)

	// Register last login
	u.SetLastLogin()
	if err := user_model.UpdateUserCols(db.DefaultContext, u, "last_login_unix"); err != nil {
		ctx.ServerError("UpdateUserCols", err)
		return setting.AppSubURL + "/"
	}

	if redirectTo := ctx.GetCookie("redirect_to"); len(redirectTo) > 0 && !utils.IsExternalURL(redirectTo) {
		middleware.DeleteRedirectToCookie(ctx.Resp)
		if obeyRedirect {
			ctx.RedirectToFirst(redirectTo)
		}
		return redirectTo
	}

	if obeyRedirect {
		ctx.Redirect(setting.AppSubURL + "/")
	}
	return setting.AppSubURL + "/"
}

func getUserName(gothUser *goth.User) string {
	switch setting.OAuth2Client.Username {
	case setting.OAuth2UsernameEmail:
		return strings.Split(gothUser.Email, "@")[0]
	case setting.OAuth2UsernameNickname:
		return gothUser.NickName
	default: // OAuth2UsernameUserid
		return gothUser.UserID
	}
}

// HandleSignOut resets the session and sets the cookies
func HandleSignOut(ctx *context.Context) {
	_ = ctx.Session.Flush()
	_ = ctx.Session.Destroy(ctx.Resp, ctx.Req)
	ctx.DeleteCookie(setting.CookieUserName)
	ctx.DeleteCookie(setting.CookieRememberName)
	middleware.DeleteCSRFCookie(ctx.Resp)
	middleware.DeleteLocaleCookie(ctx.Resp)
	middleware.DeleteRedirectToCookie(ctx.Resp)
}

// SignOut sign out from login status
func SignOut(ctx *context.Context) {
	if ctx.User != nil {
		eventsource.GetManager().SendMessageBlocking(ctx.User.ID, &eventsource.Event{
			Name: "logout",
			Data: ctx.Session.ID(),
		})
	}
	HandleSignOut(ctx)
	ctx.Redirect(setting.AppSubURL + "/")
}

// SignUp render the register page
func SignUp(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("sign_up")

	ctx.Data["SignUpLink"] = setting.AppSubURL + "/user/sign_up"

	ctx.Data["EnableCaptcha"] = setting.Service.EnableCaptcha
	ctx.Data["RecaptchaURL"] = setting.Service.RecaptchaURL
	ctx.Data["Captcha"] = context.GetImageCaptcha()
	ctx.Data["CaptchaType"] = setting.Service.CaptchaType
	ctx.Data["RecaptchaSitekey"] = setting.Service.RecaptchaSitekey
	ctx.Data["HcaptchaSitekey"] = setting.Service.HcaptchaSitekey
	ctx.Data["PageIsSignUp"] = true

	// Show Disabled Registration message if DisableRegistration or AllowOnlyExternalRegistration options are true
	ctx.Data["DisableRegistration"] = setting.Service.DisableRegistration || setting.Service.AllowOnlyExternalRegistration

	ctx.HTML(http.StatusOK, tplSignUp)
}

// SignUpPost response for sign up information submission
func SignUpPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.RegisterForm)
	ctx.Data["Title"] = ctx.Tr("sign_up")

	ctx.Data["SignUpLink"] = setting.AppSubURL + "/user/sign_up"

	ctx.Data["EnableCaptcha"] = setting.Service.EnableCaptcha
	ctx.Data["RecaptchaURL"] = setting.Service.RecaptchaURL
	ctx.Data["Captcha"] = context.GetImageCaptcha()
	ctx.Data["CaptchaType"] = setting.Service.CaptchaType
	ctx.Data["RecaptchaSitekey"] = setting.Service.RecaptchaSitekey
	ctx.Data["HcaptchaSitekey"] = setting.Service.HcaptchaSitekey
	ctx.Data["PageIsSignUp"] = true

	// Permission denied if DisableRegistration or AllowOnlyExternalRegistration options are true
	if setting.Service.DisableRegistration || setting.Service.AllowOnlyExternalRegistration {
		ctx.Error(http.StatusForbidden)
		return
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplSignUp)
		return
	}

	if setting.Service.EnableCaptcha {
		var valid bool
		var err error
		switch setting.Service.CaptchaType {
		case setting.ImageCaptcha:
			valid = context.GetImageCaptcha().VerifyReq(ctx.Req)
		case setting.ReCaptcha:
			valid, err = recaptcha.Verify(ctx, form.GRecaptchaResponse)
		case setting.HCaptcha:
			valid, err = hcaptcha.Verify(ctx, form.HcaptchaResponse)
		default:
			ctx.ServerError("Unknown Captcha Type", fmt.Errorf("Unknown Captcha Type: %s", setting.Service.CaptchaType))
			return
		}
		if err != nil {
			log.Debug("%s", err.Error())
		}

		if !valid {
			ctx.Data["Err_Captcha"] = true
			ctx.RenderWithErr(ctx.Tr("form.captcha_incorrect"), tplSignUp, &form)
			return
		}
	}

	if !form.IsEmailDomainAllowed() {
		ctx.RenderWithErr(ctx.Tr("auth.email_domain_blacklisted"), tplSignUp, &form)
		return
	}

	if form.Password != form.Retype {
		ctx.Data["Err_Password"] = true
		ctx.RenderWithErr(ctx.Tr("form.password_not_match"), tplSignUp, &form)
		return
	}
	if len(form.Password) < setting.MinPasswordLength {
		ctx.Data["Err_Password"] = true
		ctx.RenderWithErr(ctx.Tr("auth.password_too_short", setting.MinPasswordLength), tplSignUp, &form)
		return
	}
	if !password.IsComplexEnough(form.Password) {
		ctx.Data["Err_Password"] = true
		ctx.RenderWithErr(password.BuildComplexityError(ctx), tplSignUp, &form)
		return
	}
	pwned, err := password.IsPwned(ctx, form.Password)
	if pwned {
		errMsg := ctx.Tr("auth.password_pwned")
		if err != nil {
			log.Error(err.Error())
			errMsg = ctx.Tr("auth.password_pwned_err")
		}
		ctx.Data["Err_Password"] = true
		ctx.RenderWithErr(errMsg, tplSignUp, &form)
		return
	}

	u := &user_model.User{
		Name:         form.UserName,
		Email:        form.Email,
		Passwd:       form.Password,
		IsActive:     !(setting.Service.RegisterEmailConfirm || setting.Service.RegisterManualConfirm),
		IsRestricted: setting.Service.DefaultUserIsRestricted,
	}

	if !createAndHandleCreatedUser(ctx, tplSignUp, form, u, nil, false) {
		// error already handled
		return
	}

	ctx.Flash.Success(ctx.Tr("auth.sign_up_successful"))
	handleSignIn(ctx, u, false)
}

// createAndHandleCreatedUser calls createUserInContext and
// then handleUserCreated.
func createAndHandleCreatedUser(ctx *context.Context, tpl base.TplName, form interface{}, u *user_model.User, gothUser *goth.User, allowLink bool) bool {
	if !createUserInContext(ctx, tpl, form, u, gothUser, allowLink) {
		return false
	}
	return handleUserCreated(ctx, u, gothUser)
}

// createUserInContext creates a user and handles errors within a given context.
// Optionally a template can be specified.
func createUserInContext(ctx *context.Context, tpl base.TplName, form interface{}, u *user_model.User, gothUser *goth.User, allowLink bool) (ok bool) {
	if err := user_model.CreateUser(u); err != nil {
		if allowLink && (user_model.IsErrUserAlreadyExist(err) || user_model.IsErrEmailAlreadyUsed(err)) {
			if setting.OAuth2Client.AccountLinking == setting.OAuth2AccountLinkingAuto {
				var user *user_model.User
				user = &user_model.User{Name: u.Name}
				hasUser, err := user_model.GetUser(user)
				if !hasUser || err != nil {
					user = &user_model.User{Email: u.Email}
					hasUser, err = user_model.GetUser(user)
					if !hasUser || err != nil {
						ctx.ServerError("UserLinkAccount", err)
						return
					}
				}

				// TODO: probably we should respect 'remember' user's choice...
				linkAccount(ctx, user, *gothUser, true)
				return // user is already created here, all redirects are handled
			} else if setting.OAuth2Client.AccountLinking == setting.OAuth2AccountLinkingLogin {
				showLinkingLogin(ctx, *gothUser)
				return // user will be created only after linking login
			}
		}

		// handle error without template
		if len(tpl) == 0 {
			ctx.ServerError("CreateUser", err)
			return
		}

		// handle error with template
		switch {
		case user_model.IsErrUserAlreadyExist(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("form.username_been_taken"), tpl, form)
		case user_model.IsErrEmailAlreadyUsed(err):
			ctx.Data["Err_Email"] = true
			ctx.RenderWithErr(ctx.Tr("form.email_been_used"), tpl, form)
		case user_model.IsErrEmailInvalid(err):
			ctx.Data["Err_Email"] = true
			ctx.RenderWithErr(ctx.Tr("form.email_invalid"), tpl, form)
		case db.IsErrNameReserved(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("user.form.name_reserved", err.(db.ErrNameReserved).Name), tpl, form)
		case db.IsErrNamePatternNotAllowed(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("user.form.name_pattern_not_allowed", err.(db.ErrNamePatternNotAllowed).Pattern), tpl, form)
		case db.IsErrNameCharsNotAllowed(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("user.form.name_chars_not_allowed", err.(db.ErrNameCharsNotAllowed).Name), tpl, form)
		default:
			ctx.ServerError("CreateUser", err)
		}
		return
	}
	log.Trace("Account created: %s", u.Name)
	return true
}

// handleUserCreated does additional steps after a new user is created.
// It auto-sets admin for the only user, updates the optional external user and
// sends a confirmation email if required.
func handleUserCreated(ctx *context.Context, u *user_model.User, gothUser *goth.User) (ok bool) {
	// Auto-set admin for the only user.
	if user_model.CountUsers() == 1 {
		u.IsAdmin = true
		u.IsActive = true
		u.SetLastLogin()
		if err := user_model.UpdateUserCols(db.DefaultContext, u, "is_admin", "is_active", "last_login_unix"); err != nil {
			ctx.ServerError("UpdateUser", err)
			return
		}
	}

	// update external user information
	if gothUser != nil {
		if err := externalaccount.UpdateExternalUser(u, *gothUser); err != nil {
			log.Error("UpdateExternalUser failed: %v", err)
		}
	}

	// Send confirmation email
	if !u.IsActive && u.ID > 1 {
		mailer.SendActivateAccountMail(ctx.Locale, u)

		ctx.Data["IsSendRegisterMail"] = true
		ctx.Data["Email"] = u.Email
		ctx.Data["ActiveCodeLives"] = timeutil.MinutesToFriendly(setting.Service.ActiveCodeLives, ctx.Locale.Language())
		ctx.HTML(http.StatusOK, TplActivate)

		if err := ctx.Cache.Put("MailResendLimit_"+u.LowerName, u.LowerName, 180); err != nil {
			log.Error("Set cache(MailResendLimit) fail: %v", err)
		}
		return
	}

	return true
}

// Activate render activate user page
func Activate(ctx *context.Context) {
	code := ctx.FormString("code")

	if len(code) == 0 {
		ctx.Data["IsActivatePage"] = true
		if ctx.User == nil || ctx.User.IsActive {
			ctx.NotFound("invalid user", nil)
			return
		}
		// Resend confirmation email.
		if setting.Service.RegisterEmailConfirm {
			if ctx.Cache.IsExist("MailResendLimit_" + ctx.User.LowerName) {
				ctx.Data["ResendLimited"] = true
			} else {
				ctx.Data["ActiveCodeLives"] = timeutil.MinutesToFriendly(setting.Service.ActiveCodeLives, ctx.Locale.Language())
				mailer.SendActivateAccountMail(ctx.Locale, ctx.User)

				if err := ctx.Cache.Put("MailResendLimit_"+ctx.User.LowerName, ctx.User.LowerName, 180); err != nil {
					log.Error("Set cache(MailResendLimit) fail: %v", err)
				}
			}
		} else {
			ctx.Data["ServiceNotEnabled"] = true
		}
		ctx.HTML(http.StatusOK, TplActivate)
		return
	}

	user := user_model.VerifyUserActiveCode(code)
	// if code is wrong
	if user == nil {
		ctx.Data["IsActivateFailed"] = true
		ctx.HTML(http.StatusOK, TplActivate)
		return
	}

	// if account is local account, verify password
	if user.LoginSource == 0 {
		ctx.Data["Code"] = code
		ctx.Data["NeedsPassword"] = true
		ctx.HTML(http.StatusOK, TplActivate)
		return
	}

	handleAccountActivation(ctx, user)
}

// ActivatePost handles account activation with password check
func ActivatePost(ctx *context.Context) {
	code := ctx.FormString("code")
	if len(code) == 0 {
		ctx.Redirect(setting.AppSubURL + "/user/activate")
		return
	}

	user := user_model.VerifyUserActiveCode(code)
	// if code is wrong
	if user == nil {
		ctx.Data["IsActivateFailed"] = true
		ctx.HTML(http.StatusOK, TplActivate)
		return
	}

	// if account is local account, verify password
	if user.LoginSource == 0 {
		password := ctx.FormString("password")
		if len(password) == 0 {
			ctx.Data["Code"] = code
			ctx.Data["NeedsPassword"] = true
			ctx.HTML(http.StatusOK, TplActivate)
			return
		}
		if !user.ValidatePassword(password) {
			ctx.Data["IsActivateFailed"] = true
			ctx.HTML(http.StatusOK, TplActivate)
			return
		}
	}

	handleAccountActivation(ctx, user)
}

func handleAccountActivation(ctx *context.Context, user *user_model.User) {
	user.IsActive = true
	var err error
	if user.Rands, err = user_model.GetUserSalt(); err != nil {
		ctx.ServerError("UpdateUser", err)
		return
	}
	if err := user_model.UpdateUserCols(db.DefaultContext, user, "is_active", "rands"); err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.NotFound("UpdateUserCols", err)
		} else {
			ctx.ServerError("UpdateUser", err)
		}
		return
	}

	if err := user_model.ActivateUserEmail(user.ID, user.Email, true); err != nil {
		log.Error("Unable to activate email for user: %-v with email: %s: %v", user, user.Email, err)
		ctx.ServerError("ActivateUserEmail", err)
		return
	}

	log.Trace("User activated: %s", user.Name)

	if _, err := session.RegenerateSession(ctx.Resp, ctx.Req); err != nil {
		log.Error("Unable to regenerate session for user: %-v with email: %s: %v", user, user.Email, err)
		ctx.ServerError("ActivateUserEmail", err)
		return
	}

	if err := ctx.Session.Set("uid", user.ID); err != nil {
		log.Error("Error setting uid in session[%s]: %v", ctx.Session.ID(), err)
	}
	if err := ctx.Session.Set("uname", user.Name); err != nil {
		log.Error("Error setting uname in session[%s]: %v", ctx.Session.ID(), err)
	}
	if err := ctx.Session.Release(); err != nil {
		log.Error("Error storing session[%s]: %v", ctx.Session.ID(), err)
	}

	if err := resetLocale(ctx, user); err != nil {
		ctx.ServerError("resetLocale", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("auth.account_activated"))
	ctx.Redirect(setting.AppSubURL + "/")
}

// ActivateEmail render the activate email page
func ActivateEmail(ctx *context.Context) {
	code := ctx.FormString("code")
	emailStr := ctx.FormString("email")

	// Verify code.
	if email := user_model.VerifyActiveEmailCode(code, emailStr); email != nil {
		if err := user_model.ActivateEmail(email); err != nil {
			ctx.ServerError("ActivateEmail", err)
		}

		log.Trace("Email activated: %s", email.Email)
		ctx.Flash.Success(ctx.Tr("settings.add_email_success"))

		if u, err := user_model.GetUserByID(email.UID); err != nil {
			log.Warn("GetUserByID: %d", email.UID)
		} else {
			// Allow user to validate more emails
			_ = ctx.Cache.Delete("MailResendLimit_" + u.LowerName)
		}
	}

	// FIXME: e-mail verification does not require the user to be logged in,
	// so this could be redirecting to the login page.
	// Should users be logged in automatically here? (consider 2FA requirements, etc.)
	ctx.Redirect(setting.AppSubURL + "/user/settings/account")
}
