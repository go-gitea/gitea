// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/auth/password"
	"code.gitea.io/gitea/modules/eventsource"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/session"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/modules/web/middleware"
	auth_service "code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/auth/source/oauth2"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/externalaccount"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/mailer"
	user_service "code.gitea.io/gitea/services/user"

	"github.com/markbates/goth"
)

const (
	tplSignIn         templates.TplName = "user/auth/signin"          // for sign in page
	tplSignUp         templates.TplName = "user/auth/signup"          // for sign up page
	TplActivate       templates.TplName = "user/auth/activate"        // for activate user
	TplActivatePrompt templates.TplName = "user/auth/activate_prompt" // for showing a message for user activation
)

// autoSignIn reads cookie and try to auto-login.
func autoSignIn(ctx *context.Context) (bool, error) {
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

	ctx.Csrf.PrepareForSessionUser(ctx)
	return true, nil
}

func resetLocale(ctx *context.Context, u *user_model.User) error {
	// Language setting of the user overwrites the one previously set
	// If the user does not have a locale set, we save the current one.
	if u.Language == "" {
		opts := &user_service.UpdateOptions{
			Language: optional.Some(ctx.Locale.Language()),
		}
		if err := user_service.UpdateUser(ctx, u, opts); err != nil {
			return err
		}
	}

	middleware.SetLocaleCookie(ctx.Resp, u.Language, 0)

	if ctx.Locale.Language() != u.Language {
		ctx.Locale = middleware.Locale(ctx.Resp, ctx.Req)
	}

	return nil
}

func RedirectAfterLogin(ctx *context.Context) {
	redirectTo := ctx.FormString("redirect_to")
	if redirectTo == "" {
		redirectTo = ctx.GetSiteCookie("redirect_to")
	}
	middleware.DeleteRedirectToCookie(ctx.Resp)
	nextRedirectTo := setting.AppSubURL + string(setting.LandingPageURL)
	if setting.LandingPageURL == setting.LandingPageLogin {
		nextRedirectTo = setting.AppSubURL + "/" // do not cycle-redirect to the login page
	}
	ctx.RedirectToCurrentSite(redirectTo, nextRedirectTo)
}

func CheckAutoLogin(ctx *context.Context) bool {
	isSucceed, err := autoSignIn(ctx) // try to auto-login
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
	}

	if isSucceed {
		RedirectAfterLogin(ctx)
		return true
	}

	return false
}

func prepareSignInPageData(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("sign_in")
	ctx.Data["OAuth2Providers"], _ = oauth2.GetOAuth2Providers(ctx, optional.Some(true))
	ctx.Data["Title"] = ctx.Tr("sign_in")
	ctx.Data["SignInLink"] = setting.AppSubURL + "/user/login"
	ctx.Data["PageIsSignIn"] = true
	ctx.Data["PageIsLogin"] = true
	ctx.Data["EnableSSPI"] = auth.IsSSPIEnabled(ctx)
	ctx.Data["EnablePasswordSignInForm"] = setting.Service.EnablePasswordSignInForm

	if setting.Service.EnableCaptcha && setting.Service.RequireCaptchaForLogin {
		context.SetCaptchaData(ctx)
	}
}

// SignIn render sign in page
func SignIn(ctx *context.Context) {
	if CheckAutoLogin(ctx) {
		return
	}
	if ctx.IsSigned {
		RedirectAfterLogin(ctx)
		return
	}
	prepareSignInPageData(ctx)
	ctx.HTML(http.StatusOK, tplSignIn)
}

// SignInPost response for sign in request
func SignInPost(ctx *context.Context) {
	if !setting.Service.EnablePasswordSignInForm {
		ctx.Error(http.StatusForbidden)
		return
	}

	prepareSignInPageData(ctx)
	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplSignIn)
		return
	}

	form := web.GetForm(ctx).(*forms.SignInForm)

	if setting.Service.EnableCaptcha && setting.Service.RequireCaptchaForLogin {
		context.VerifyCaptcha(ctx, tplSignIn, form)
		if ctx.Written() {
			return
		}
	}

	u, source, err := auth_service.UserSignIn(ctx, form.UserName, form.Password)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) || errors.Is(err, util.ErrInvalidArgument) {
			ctx.RenderWithErr(ctx.Tr("form.username_password_incorrect"), tplSignIn, &form)
			log.Warn("Failed authentication attempt for %s from %s: %v", form.UserName, ctx.RemoteAddr(), err)
		} else if user_model.IsErrEmailAlreadyUsed(err) {
			ctx.RenderWithErr(ctx.Tr("form.email_been_used"), tplSignIn, &form)
			log.Warn("Failed authentication attempt for %s from %s: %v", form.UserName, ctx.RemoteAddr(), err)
		} else if user_model.IsErrUserProhibitLogin(err) {
			log.Warn("Failed authentication attempt for %s from %s: %v", form.UserName, ctx.RemoteAddr(), err)
			ctx.Data["Title"] = ctx.Tr("auth.prohibit_login")
			ctx.HTML(http.StatusOK, "user/auth/prohibit_login")
		} else if user_model.IsErrUserInactive(err) {
			if setting.Service.RegisterEmailConfirm {
				ctx.Data["Title"] = ctx.Tr("auth.active_your_account")
				ctx.HTML(http.StatusOK, TplActivate)
			} else {
				log.Warn("Failed authentication attempt for %s from %s: %v", form.UserName, ctx.RemoteAddr(), err)
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
	if u.Language == "" {
		opts := &user_service.UpdateOptions{
			Language: optional.Some(ctx.Locale.Language()),
		}
		if err := user_service.UpdateUser(ctx, u, opts); err != nil {
			ctx.ServerError("UpdateUser Language", fmt.Errorf("Error updating user language [user: %d, locale: %s]", u.ID, ctx.Locale.Language()))
			return setting.AppSubURL + "/"
		}
	}

	middleware.SetLocaleCookie(ctx.Resp, u.Language, 0)

	if ctx.Locale.Language() != u.Language {
		ctx.Locale = middleware.Locale(ctx.Resp, ctx.Req)
	}

	// force to generate a new CSRF token
	ctx.Csrf.PrepareForSessionUser(ctx)

	// Register last login
	if err := user_service.UpdateUser(ctx, u, &user_service.UpdateOptions{SetLastLogin: true}); err != nil {
		ctx.ServerError("UpdateUser", err)
		return setting.AppSubURL + "/"
	}

	if redirectTo := ctx.GetSiteCookie("redirect_to"); redirectTo != "" && httplib.IsCurrentGiteaSiteURL(ctx, redirectTo) {
		middleware.DeleteRedirectToCookie(ctx.Resp)
		if obeyRedirect {
			ctx.RedirectToCurrentSite(redirectTo)
		}
		return redirectTo
	}

	if obeyRedirect {
		ctx.Redirect(setting.AppSubURL + "/")
	}
	return setting.AppSubURL + "/"
}

// extractUserNameFromOAuth2 tries to extract a normalized username from the given OAuth2 user.
// It returns ("", nil) if the required field doesn't exist.
func extractUserNameFromOAuth2(gothUser *goth.User) (string, error) {
	switch setting.OAuth2Client.Username {
	case setting.OAuth2UsernameEmail:
		return user_model.NormalizeUserName(gothUser.Email)
	case setting.OAuth2UsernamePreferredUsername:
		if preferredUsername, ok := gothUser.RawData["preferred_username"].(string); ok {
			return user_model.NormalizeUserName(preferredUsername)
		}
		return "", nil
	case setting.OAuth2UsernameNickname:
		return user_model.NormalizeUserName(gothUser.NickName)
	default: // OAuth2UsernameUserid
		return gothUser.UserID, nil
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

	oauth2Providers, err := oauth2.GetOAuth2Providers(ctx, optional.Some(true))
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

	oauth2Providers, err := oauth2.GetOAuth2Providers(ctx, optional.Some(true))
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
	if err := password.IsPwned(ctx, form.Password); err != nil {
		errMsg := ctx.Tr("auth.password_pwned", "https://haveibeenpwned.com/Passwords")
		if password.IsErrIsPwnedRequest(err) {
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
func createAndHandleCreatedUser(ctx *context.Context, tpl templates.TplName, form any, u *user_model.User, overwrites *user_model.CreateUserOverwriteOptions, gothUser *goth.User, allowLink bool) bool {
	if !createUserInContext(ctx, tpl, form, u, overwrites, gothUser, allowLink) {
		return false
	}
	return handleUserCreated(ctx, u, gothUser)
}

// createUserInContext creates a user and handles errors within a given context.
// Optionally a template can be specified.
func createUserInContext(ctx *context.Context, tpl templates.TplName, form any, u *user_model.User, overwrites *user_model.CreateUserOverwriteOptions, gothUser *goth.User, allowLink bool) (ok bool) {
	meta := &user_model.Meta{
		InitialIP:        ctx.RemoteAddr(),
		InitialUserAgent: ctx.Req.UserAgent(),
	}
	if err := user_model.CreateUser(ctx, u, meta, overwrites); err != nil {
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
		opts := &user_service.UpdateOptions{
			IsActive:     optional.Some(true),
			IsAdmin:      optional.Some(true),
			SetLastLogin: true,
		}
		if err := user_service.UpdateUser(ctx, u, opts); err != nil {
			ctx.ServerError("UpdateUser", err)
			return false
		}
	}

	// update external user information
	if gothUser != nil {
		if err := externalaccount.EnsureLinkExternalToUser(ctx, u, *gothUser); err != nil {
			log.Error("EnsureLinkExternalToUser failed: %v", err)
		}
	}

	// for active user or the first (admin) user, we don't need to send confirmation email
	if u.IsActive || u.ID == 1 {
		return true
	}

	if setting.Service.RegisterManualConfirm {
		renderActivationPromptMessage(ctx, ctx.Locale.Tr("auth.manual_activation_only"))
		return false
	}

	sendActivateEmail(ctx, u)
	return false
}

func renderActivationPromptMessage(ctx *context.Context, msg template.HTML) {
	ctx.Data["ActivationPromptMessage"] = msg
	ctx.HTML(http.StatusOK, TplActivatePrompt)
}

func sendActivateEmail(ctx *context.Context, u *user_model.User) {
	if ctx.Cache.IsExist("MailResendLimit_" + u.LowerName) {
		renderActivationPromptMessage(ctx, ctx.Locale.Tr("auth.resent_limit_prompt"))
		return
	}

	if err := ctx.Cache.Put("MailResendLimit_"+u.LowerName, u.LowerName, 180); err != nil {
		log.Error("Set cache(MailResendLimit) fail: %v", err)
		renderActivationPromptMessage(ctx, ctx.Locale.Tr("auth.resent_limit_prompt"))
		return
	}

	mailer.SendActivateAccountMail(ctx.Locale, u)

	activeCodeLives := timeutil.MinutesToFriendly(setting.Service.ActiveCodeLives, ctx.Locale)
	msgHTML := ctx.Locale.Tr("auth.confirmation_mail_sent_prompt_ex", u.Email, activeCodeLives)
	renderActivationPromptMessage(ctx, msgHTML)
}

func renderActivationVerifyPassword(ctx *context.Context, code string) {
	ctx.Data["ActivationCode"] = code
	ctx.Data["NeedVerifyLocalPassword"] = true
	ctx.HTML(http.StatusOK, TplActivate)
}

func renderActivationChangeEmail(ctx *context.Context) {
	ctx.HTML(http.StatusOK, TplActivate)
}

// Activate render activate user page
func Activate(ctx *context.Context) {
	code := ctx.FormString("code")

	if code == "" {
		if ctx.Doer == nil {
			ctx.Redirect(setting.AppSubURL + "/user/login")
			return
		} else if ctx.Doer.IsActive {
			ctx.Redirect(setting.AppSubURL + "/")
			return
		}

		if setting.MailService == nil || !setting.Service.RegisterEmailConfirm {
			renderActivationPromptMessage(ctx, ctx.Tr("auth.disable_register_mail"))
			return
		}

		// Resend confirmation email. FIXME: ideally this should be in a POST request
		sendActivateEmail(ctx, ctx.Doer)
		return
	}

	// TODO: ctx.Doer/ctx.Data["SignedUser"] could be nil or not the same user as the one being activated
	user := user_model.VerifyUserTimeLimitCode(ctx, &user_model.TimeLimitCodeOptions{Purpose: user_model.TimeLimitCodeActivateAccount}, code)
	if user == nil { // if code is wrong
		renderActivationPromptMessage(ctx, ctx.Locale.Tr("auth.invalid_code"))
		return
	}

	// if account is local account, verify password
	if user.LoginSource == 0 {
		renderActivationVerifyPassword(ctx, code)
		return
	}

	handleAccountActivation(ctx, user)
}

// ActivatePost handles account activation with password check
func ActivatePost(ctx *context.Context) {
	code := ctx.FormString("code")
	if ctx.Doer != nil && ctx.Doer.IsActive {
		ctx.Redirect(setting.AppSubURL + "/user/activate") // it will redirect again to the correct page
		return
	}

	if code == "" {
		newEmail := strings.TrimSpace(ctx.FormString("change_email"))
		if ctx.Doer != nil && newEmail != "" && !strings.EqualFold(ctx.Doer.Email, newEmail) {
			if user_model.ValidateEmail(newEmail) != nil {
				ctx.Flash.Error(ctx.Locale.Tr("form.email_invalid"), true)
				renderActivationChangeEmail(ctx)
				return
			}
			err := user_model.ChangeInactivePrimaryEmail(ctx, ctx.Doer.ID, ctx.Doer.Email, newEmail)
			if err != nil {
				ctx.Flash.Error(ctx.Locale.Tr("admin.emails.not_updated", newEmail), true)
				renderActivationChangeEmail(ctx)
				return
			}
			ctx.Doer.Email = newEmail
		}
		// FIXME: at the moment, GET request handles the "send confirmation email" action. But the old code does this redirect and then send a confirmation email.
		ctx.Redirect(setting.AppSubURL + "/user/activate")
		return
	}

	// TODO: ctx.Doer/ctx.Data["SignedUser"] could be nil or not the same user as the one being activated
	user := user_model.VerifyUserTimeLimitCode(ctx, &user_model.TimeLimitCodeOptions{Purpose: user_model.TimeLimitCodeActivateAccount}, code)
	if user == nil { // if code is wrong
		renderActivationPromptMessage(ctx, ctx.Locale.Tr("auth.invalid_code"))
		return
	}

	// if account is local account, verify password
	if user.LoginSource == 0 {
		password := ctx.FormString("password")
		if password == "" {
			renderActivationVerifyPassword(ctx, code)
			return
		}
		if !user.ValidatePassword(password) {
			ctx.Flash.Error(ctx.Locale.Tr("auth.invalid_password"), true)
			renderActivationVerifyPassword(ctx, code)
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

	ctx.Csrf.PrepareForSessionUser(ctx)

	if err := resetLocale(ctx, user); err != nil {
		ctx.ServerError("resetLocale", err)
		return
	}

	if err := user_service.UpdateUser(ctx, user, &user_service.UpdateOptions{SetLastLogin: true}); err != nil {
		ctx.ServerError("UpdateUser", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("auth.account_activated"))
	if redirectTo := ctx.GetSiteCookie("redirect_to"); len(redirectTo) > 0 {
		middleware.DeleteRedirectToCookie(ctx.Resp)
		ctx.RedirectToCurrentSite(redirectTo)
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
			return
		}

		log.Trace("Email activated: %s", email.Email)
		ctx.Flash.Success(ctx.Tr("settings.add_email_success"))

		if u, err := user_model.GetUserByID(ctx, email.UID); err != nil {
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
