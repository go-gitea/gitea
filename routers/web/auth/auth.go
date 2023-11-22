// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/auth/password"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/eventsource"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/session"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
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

// autoSignIn reads cookie and try to auto-login.
func autoSignIn(ctx *context.Context) (bool, error) {
	if !db.HasEngine {
		return false, nil
	}

	isSucceed := false
	defer func() {
		if !isSucceed {
			ctx.DeleteSiteCookie(setting.CookieRememberName)
		}
	}()

	if err := auth.DeleteExpiredAuthTokens(ctx); err != nil {
		log.Error("Failed to delete expired auth tokens: %v", err)
	}

	t, err := auth_service.CheckAuthToken(ctx, ctx.GetSiteCookie(setting.CookieRememberName))
	if err != nil {
		switch err {
		case auth_service.ErrAuthTokenInvalidFormat, auth_service.ErrAuthTokenExpired:
			return false, nil
		}
		return false, err
	}
	if t == nil {
		return false, nil
	}

	u, err := user_model.GetUserByID(ctx, t.UserID)
	if err != nil {
		if !user_model.IsErrUserNotExist(err) {
			return false, fmt.Errorf("GetUserByID: %w", err)
		}
		return false, nil
	}

	isSucceed = true

	nt, token, err := auth_service.RegenerateAuthToken(ctx, t)
	if err != nil {
		return false, err
	}

	ctx.SetSiteCookie(setting.CookieRememberName, nt.ID+":"+token, setting.LogInRememberDays*timeutil.Day)

	if err := updateSession(ctx, nil, map[string]any{
		// Set session IDs
		"uid":   u.ID,
		"uname": u.Name,
	}); err != nil {
		return false, fmt.Errorf("unable to updateSession: %w", err)
	}

	if err := resetLocale(ctx, u); err != nil {
		return false, err
	}

	ctx.Csrf.DeleteCookie(ctx)
	return true, nil
}

func resetLocale(ctx *context.Context, u *user_model.User) error {
	// Language setting of the user overwrites the one previously set
	// If the user does not have a locale set, we save the current one.
	if len(u.Language) == 0 {
		u.Language = ctx.Locale.Language()
		if err := user_model.UpdateUserCols(ctx, u, "language"); err != nil {
			return err
		}
	}

	middleware.SetLocaleCookie(ctx.Resp, u.Language, 0)

	if ctx.Locale.Language() != u.Language {
		ctx.Locale = middleware.Locale(ctx.Resp, ctx.Req)
	}

	return nil
}

func CheckAutoLogin(ctx *context.Context) bool {
	// Check auto-login
	isSucceed, err := autoSignIn(ctx)
	if err != nil {
		if errors.Is(err, auth_service.ErrAuthTokenInvalidHash) {
			ctx.Flash.Error(ctx.Tr("auth.remember_me.compromised"), true)
			return false
		}
		ctx.ServerError("autoSignIn", err)
		return true
	}

	redirectTo := ctx.FormString("redirect_to")
	if len(redirectTo) > 0 {
		middleware.SetRedirectToCookie(ctx.Resp, redirectTo)
	} else {
		redirectTo = ctx.GetSiteCookie("redirect_to")
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

	if CheckAutoLogin(ctx) {
		return
	}

	oauth2Providers, err := oauth2.GetOAuth2Providers(ctx, util.OptionalBoolTrue)
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}
	ctx.Data["OAuth2Providers"] = oauth2Providers
	ctx.Data["Title"] = ctx.Tr("sign_in")
	ctx.Data["SignInLink"] = setting.AppSubURL + "/user/login"
	ctx.Data["PageIsSignIn"] = true
	ctx.Data["PageIsLogin"] = true
	ctx.Data["EnableSSPI"] = auth.IsSSPIEnabled(ctx)

	if setting.Service.EnableCaptcha && setting.Service.RequireCaptchaForLogin {
		context.SetCaptchaData(ctx)
	}

	ctx.HTML(http.StatusOK, tplSignIn)
}

// SignInPost response for sign in request
func SignInPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("sign_in")

	oauth2Providers, err := oauth2.GetOAuth2Providers(ctx, util.OptionalBoolTrue)
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}
	ctx.Data["OAuth2Providers"] = oauth2Providers
	ctx.Data["Title"] = ctx.Tr("sign_in")
	ctx.Data["SignInLink"] = setting.AppSubURL + "/user/login"
	ctx.Data["PageIsSignIn"] = true
	ctx.Data["PageIsLogin"] = true
	ctx.Data["EnableSSPI"] = auth.IsSSPIEnabled(ctx)

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplSignIn)
		return
	}

	form := web.GetForm(ctx).(*forms.SignInForm)

	if setting.Service.EnableCaptcha && setting.Service.RequireCaptchaForLogin {
		context.SetCaptchaData(ctx)

		context.VerifyCaptcha(ctx, tplSignIn, form)
		if ctx.Written() {
			return
		}
	}

	u, source, err := auth_service.UserSignIn(ctx, form.UserName, form.Password)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) || errors.Is(err, util.ErrInvalidArgument) {
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
	hasTOTPtwofa, err := auth.HasTwoFactorByUID(ctx, u.ID)
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}

	// Check if the user has webauthn registration
	hasWebAuthnTwofa, err := auth.HasWebAuthnRegistrationsByUID(ctx, u.ID)
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}

	if !hasTOTPtwofa && !hasWebAuthnTwofa {
		// No two factor auth configured we can sign in the user
		handleSignIn(ctx, u, form.Remember)
		return
	}

	updates := map[string]any{
		// User will need to use 2FA TOTP or WebAuthn, save data
		"twofaUid":      u.ID,
		"twofaRemember": form.Remember,
	}
	if hasTOTPtwofa {
		// User will need to use WebAuthn, save data
		updates["totpEnrolled"] = u.ID
	}
	if err := updateSession(ctx, nil, updates); err != nil {
		ctx.ServerError("UserSignIn: Unable to update session", err)
		return
	}

	// If we have WebAuthn redirect there first
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
		nt, token, err := auth_service.CreateAuthTokenForUserID(ctx, u.ID)
		if err != nil {
			ctx.ServerError("CreateAuthTokenForUserID", err)
			return setting.AppSubURL + "/"
		}

		ctx.SetSiteCookie(setting.CookieRememberName, nt.ID+":"+token, setting.LogInRememberDays*timeutil.Day)
	}

	if err := updateSession(ctx, []string{
		// Delete the openid, 2fa and linkaccount data
		"openid_verified_uri",
		"openid_signin_remember",
		"openid_determined_email",
		"openid_determined_username",
		"twofaUid",
		"twofaRemember",
		"linkAccount",
	}, map[string]any{
		"uid":   u.ID,
		"uname": u.Name,
	}); err != nil {
		ctx.ServerError("RegenerateSession", err)
		return setting.AppSubURL + "/"
	}

	// Language setting of the user overwrites the one previously set
	// If the user does not have a locale set, we save the current one.
	if len(u.Language) == 0 {
		u.Language = ctx.Locale.Language()
		if err := user_model.UpdateUserCols(ctx, u, "language"); err != nil {
			ctx.ServerError("UpdateUserCols Language", fmt.Errorf("Error updating user language [user: %d, locale: %s]", u.ID, u.Language))
			return setting.AppSubURL + "/"
		}
	}

	middleware.SetLocaleCookie(ctx.Resp, u.Language, 0)

	if ctx.Locale.Language() != u.Language {
		ctx.Locale = middleware.Locale(ctx.Resp, ctx.Req)
	}

	// Clear whatever CSRF cookie has right now, force to generate a new one
	ctx.Csrf.DeleteCookie(ctx)

	// Register last login
	u.SetLastLogin()
	if err := user_model.UpdateUserCols(ctx, u, "last_login_unix"); err != nil {
		ctx.ServerError("UpdateUserCols", err)
		return setting.AppSubURL + "/"
	}

	if redirectTo := ctx.GetSiteCookie("redirect_to"); len(redirectTo) > 0 && !utils.IsExternalURL(redirectTo) {
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
	ctx.DeleteSiteCookie(setting.CookieRememberName)
	ctx.Csrf.DeleteCookie(ctx)
	middleware.DeleteRedirectToCookie(ctx.Resp)
}

// SignOut sign out from login status
func SignOut(ctx *context.Context) {
	if ctx.Doer != nil {
		eventsource.GetManager().SendMessageBlocking(ctx.Doer.ID, &eventsource.Event{
			Name: "logout",
			Data: ctx.Session.ID(),
		})
	}
	HandleSignOut(ctx)
	ctx.JSONRedirect(setting.AppSubURL + "/")
}

// SignUp render the register page
func SignUp(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("sign_up")

	ctx.Data["SignUpLink"] = setting.AppSubURL + "/user/sign_up"

	oauth2Providers, err := oauth2.GetOAuth2Providers(ctx, util.OptionalBoolTrue)
	if err != nil {
		ctx.ServerError("UserSignUp", err)
		return
	}

	ctx.Data["OAuth2Providers"] = oauth2Providers
	context.SetCaptchaData(ctx)

	ctx.Data["PageIsSignUp"] = true

	// Show Disabled Registration message if DisableRegistration or AllowOnlyExternalRegistration options are true
	ctx.Data["DisableRegistration"] = setting.Service.DisableRegistration || setting.Service.AllowOnlyExternalRegistration

	redirectTo := ctx.FormString("redirect_to")
	if len(redirectTo) > 0 {
		middleware.SetRedirectToCookie(ctx.Resp, redirectTo)
	}

	ctx.HTML(http.StatusOK, tplSignUp)
}

// SignUpPost response for sign up information submission
func SignUpPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.RegisterForm)
	ctx.Data["Title"] = ctx.Tr("sign_up")

	ctx.Data["SignUpLink"] = setting.AppSubURL + "/user/sign_up"

	oauth2Providers, err := oauth2.GetOAuth2Providers(ctx, util.OptionalBoolTrue)
	if err != nil {
		ctx.ServerError("UserSignUp", err)
		return
	}

	ctx.Data["OAuth2Providers"] = oauth2Providers
	context.SetCaptchaData(ctx)

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

	context.VerifyCaptcha(ctx, tplSignUp, form)
	if ctx.Written() {
		return
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
		ctx.RenderWithErr(password.BuildComplexityError(ctx.Locale), tplSignUp, &form)
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
		Name:   form.UserName,
		Email:  form.Email,
		Passwd: form.Password,
	}

	if !createAndHandleCreatedUser(ctx, tplSignUp, form, u, nil, nil, false) {
		// error already handled
		return
	}

	ctx.Flash.Success(ctx.Tr("auth.sign_up_successful"))
	handleSignIn(ctx, u, false)
}

// createAndHandleCreatedUser calls createUserInContext and
// then handleUserCreated.
func createAndHandleCreatedUser(ctx *context.Context, tpl base.TplName, form any, u *user_model.User, overwrites *user_model.CreateUserOverwriteOptions, gothUser *goth.User, allowLink bool) bool {
	if !createUserInContext(ctx, tpl, form, u, overwrites, gothUser, allowLink) {
		return false
	}
	return handleUserCreated(ctx, u, gothUser)
}

// createUserInContext creates a user and handles errors within a given context.
// Optionally a template can be specified.
func createUserInContext(ctx *context.Context, tpl base.TplName, form any, u *user_model.User, overwrites *user_model.CreateUserOverwriteOptions, gothUser *goth.User, allowLink bool) (ok bool) {
	if err := user_model.CreateUser(ctx, u, overwrites); err != nil {
		if allowLink && (user_model.IsErrUserAlreadyExist(err) || user_model.IsErrEmailAlreadyUsed(err)) {
			if setting.OAuth2Client.AccountLinking == setting.OAuth2AccountLinkingAuto {
				var user *user_model.User
				user = &user_model.User{Name: u.Name}
				hasUser, err := user_model.GetUser(ctx, user)
				if !hasUser || err != nil {
					user = &user_model.User{Email: u.Email}
					hasUser, err = user_model.GetUser(ctx, user)
					if !hasUser || err != nil {
						ctx.ServerError("UserLinkAccount", err)
						return false
					}
				}

				// TODO: probably we should respect 'remember' user's choice...
				linkAccount(ctx, user, *gothUser, true)
				return false // user is already created here, all redirects are handled
			} else if setting.OAuth2Client.AccountLinking == setting.OAuth2AccountLinkingLogin {
				showLinkingLogin(ctx, *gothUser)
				return false // user will be created only after linking login
			}
		}

		// handle error without template
		if len(tpl) == 0 {
			ctx.ServerError("CreateUser", err)
			return false
		}

		// handle error with template
		switch {
		case user_model.IsErrUserAlreadyExist(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("form.username_been_taken"), tpl, form)
		case user_model.IsErrEmailAlreadyUsed(err):
			ctx.Data["Err_Email"] = true
			ctx.RenderWithErr(ctx.Tr("form.email_been_used"), tpl, form)
		case user_model.IsErrEmailCharIsNotSupported(err):
			ctx.Data["Err_Email"] = true
			ctx.RenderWithErr(ctx.Tr("form.email_invalid"), tpl, form)
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
		return false
	}
	log.Trace("Account created: %s", u.Name)
	return true
}

// handleUserCreated does additional steps after a new user is created.
// It auto-sets admin for the only user, updates the optional external user and
// sends a confirmation email if required.
func handleUserCreated(ctx *context.Context, u *user_model.User, gothUser *goth.User) (ok bool) {
	// Auto-set admin for the only user.
	if user_model.CountUsers(ctx, nil) == 1 {
		u.IsAdmin = true
		u.IsActive = true
		u.SetLastLogin()
		if err := user_model.UpdateUserCols(ctx, u, "is_admin", "is_active", "last_login_unix"); err != nil {
			ctx.ServerError("UpdateUser", err)
			return false
		}
	}

	// update external user information
	if gothUser != nil {
		if err := externalaccount.UpdateExternalUser(ctx, u, *gothUser); err != nil {
			if !errors.Is(err, util.ErrNotExist) {
				log.Error("UpdateExternalUser failed: %v", err)
			}
		}
	}

	// Send confirmation email
	if !u.IsActive && u.ID > 1 {
		if setting.Service.RegisterManualConfirm {
			ctx.Data["ManualActivationOnly"] = true
			ctx.HTML(http.StatusOK, TplActivate)
			return false
		}

		mailer.SendActivateAccountMail(ctx.Locale, u)

		ctx.Data["IsSendRegisterMail"] = true
		ctx.Data["Email"] = u.Email
		ctx.Data["ActiveCodeLives"] = timeutil.MinutesToFriendly(setting.Service.ActiveCodeLives, ctx.Locale)
		ctx.HTML(http.StatusOK, TplActivate)

		if setting.CacheService.Enabled {
			if err := ctx.Cache.Put("MailResendLimit_"+u.LowerName, u.LowerName, 180); err != nil {
				log.Error("Set cache(MailResendLimit) fail: %v", err)
			}
		}
		return false
	}

	return true
}

// Activate render activate user page
func Activate(ctx *context.Context) {
	code := ctx.FormString("code")

	if len(code) == 0 {
		ctx.Data["IsActivatePage"] = true
		if ctx.Doer == nil || ctx.Doer.IsActive {
			ctx.NotFound("invalid user", nil)
			return
		}
		// Resend confirmation email.
		if setting.Service.RegisterEmailConfirm {
			if setting.CacheService.Enabled && ctx.Cache.IsExist("MailResendLimit_"+ctx.Doer.LowerName) {
				ctx.Data["ResendLimited"] = true
			} else {
				ctx.Data["ActiveCodeLives"] = timeutil.MinutesToFriendly(setting.Service.ActiveCodeLives, ctx.Locale)
				mailer.SendActivateAccountMail(ctx.Locale, ctx.Doer)

				if setting.CacheService.Enabled {
					if err := ctx.Cache.Put("MailResendLimit_"+ctx.Doer.LowerName, ctx.Doer.LowerName, 180); err != nil {
						log.Error("Set cache(MailResendLimit) fail: %v", err)
					}
				}
			}
		} else {
			ctx.Data["ServiceNotEnabled"] = true
		}
		ctx.HTML(http.StatusOK, TplActivate)
		return
	}

	user := user_model.VerifyUserActiveCode(ctx, code)
	// if code is wrong
	if user == nil {
		ctx.Data["IsCodeInvalid"] = true
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

	user := user_model.VerifyUserActiveCode(ctx, code)
	// if code is wrong
	if user == nil {
		ctx.Data["IsCodeInvalid"] = true
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
			ctx.Data["IsPasswordInvalid"] = true
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
	if err := user_model.UpdateUserCols(ctx, user, "is_active", "rands"); err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.NotFound("UpdateUserCols", err)
		} else {
			ctx.ServerError("UpdateUser", err)
		}
		return
	}

	if err := user_model.ActivateUserEmail(ctx, user.ID, user.Email, true); err != nil {
		log.Error("Unable to activate email for user: %-v with email: %s: %v", user, user.Email, err)
		ctx.ServerError("ActivateUserEmail", err)
		return
	}

	log.Trace("User activated: %s", user.Name)

	if err := updateSession(ctx, nil, map[string]any{
		"uid":   user.ID,
		"uname": user.Name,
	}); err != nil {
		log.Error("Unable to regenerate session for user: %-v with email: %s: %v", user, user.Email, err)
		ctx.ServerError("ActivateUserEmail", err)
		return
	}

	if err := resetLocale(ctx, user); err != nil {
		ctx.ServerError("resetLocale", err)
		return
	}

	// Register last login
	user.SetLastLogin()
	if err := user_model.UpdateUserCols(ctx, user, "last_login_unix"); err != nil {
		ctx.ServerError("UpdateUserCols", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("auth.account_activated"))
	if redirectTo := ctx.GetSiteCookie("redirect_to"); len(redirectTo) > 0 {
		middleware.DeleteRedirectToCookie(ctx.Resp)
		ctx.RedirectToFirst(redirectTo)
		return
	}

	ctx.Redirect(setting.AppSubURL + "/")
}

// ActivateEmail render the activate email page
func ActivateEmail(ctx *context.Context) {
	code := ctx.FormString("code")
	emailStr := ctx.FormString("email")

	// Verify code.
	if email := user_model.VerifyActiveEmailCode(ctx, code, emailStr); email != nil {
		if err := user_model.ActivateEmail(ctx, email); err != nil {
			ctx.ServerError("ActivateEmail", err)
		}

		log.Trace("Email activated: %s", email.Email)
		ctx.Flash.Success(ctx.Tr("settings.add_email_success"))

		if u, err := user_model.GetUserByID(ctx, email.UID); err != nil {
			log.Warn("GetUserByID: %d", email.UID)
		} else if setting.CacheService.Enabled {
			// Allow user to validate more emails
			_ = ctx.Cache.Delete("MailResendLimit_" + u.LowerName)
		}
	}

	// FIXME: e-mail verification does not require the user to be logged in,
	// so this could be redirecting to the login page.
	// Should users be logged in automatically here? (consider 2FA requirements, etc.)
	ctx.Redirect(setting.AppSubURL + "/user/settings/account")
}

func updateSession(ctx *context.Context, deletes []string, updates map[string]any) error {
	if _, err := session.RegenerateSession(ctx.Resp, ctx.Req); err != nil {
		return fmt.Errorf("regenerate session: %w", err)
	}
	sess := ctx.Session
	sessID := sess.ID()
	for _, k := range deletes {
		if err := sess.Delete(k); err != nil {
			return fmt.Errorf("delete %v in session[%s]: %w", k, sessID, err)
		}
	}
	for k, v := range updates {
		if err := sess.Set(k, v); err != nil {
			return fmt.Errorf("set %v in session[%s]: %w", k, sessID, err)
		}
	}
	if err := sess.Release(); err != nil {
		return fmt.Errorf("store session[%s]: %w", sessID, err)
	}
	return nil
}
