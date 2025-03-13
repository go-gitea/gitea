// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	auth_service "code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/auth/source/oauth2"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/externalaccount"
	"code.gitea.io/gitea/services/forms"

	"github.com/markbates/goth"
)

var tplLinkAccount templates.TplName = "user/auth/link_account"

// LinkAccount shows the page where the user can decide to login or create a new account
func LinkAccount(ctx *context.Context) {
	// FIXME: these common template variables should be prepared in one common function, but not just copy-paste again and again.
	ctx.Data["DisablePassword"] = !setting.Service.RequireExternalRegistrationPassword || setting.Service.AllowOnlyExternalRegistration
	ctx.Data["Title"] = ctx.Tr("link_account")
	ctx.Data["LinkAccountMode"] = true
	ctx.Data["EnableCaptcha"] = setting.Service.EnableCaptcha && setting.Service.RequireExternalRegistrationCaptcha
	ctx.Data["Captcha"] = context.GetImageCaptcha()
	ctx.Data["CaptchaType"] = setting.Service.CaptchaType
	ctx.Data["RecaptchaURL"] = setting.Service.RecaptchaURL
	ctx.Data["RecaptchaSitekey"] = setting.Service.RecaptchaSitekey
	ctx.Data["HcaptchaSitekey"] = setting.Service.HcaptchaSitekey
	ctx.Data["McaptchaSitekey"] = setting.Service.McaptchaSitekey
	ctx.Data["McaptchaURL"] = setting.Service.McaptchaURL
	ctx.Data["CfTurnstileSitekey"] = setting.Service.CfTurnstileSitekey
	ctx.Data["DisableRegistration"] = setting.Service.DisableRegistration
	ctx.Data["AllowOnlyInternalRegistration"] = setting.Service.AllowOnlyInternalRegistration
	ctx.Data["EnablePasswordSignInForm"] = setting.Service.EnablePasswordSignInForm
	ctx.Data["ShowRegistrationButton"] = false
	ctx.Data["EnablePasskeyAuth"] = setting.Service.EnablePasskeyAuth

	// use this to set the right link into the signIn and signUp templates in the link_account template
	ctx.Data["SignInLink"] = setting.AppSubURL + "/user/link_account_signin"
	ctx.Data["SignUpLink"] = setting.AppSubURL + "/user/link_account_signup"

	gothUser, ok := ctx.Session.Get("linkAccountGothUser").(goth.User)

	// If you'd like to quickly debug the "link account" page layout, just uncomment the blow line
	// Don't worry, when the below line exists, the lint won't pass: ineffectual assignment to gothUser (ineffassign)
	// gothUser, ok = goth.User{Email: "invalid-email", Name: "."}, true // intentionally use invalid data to avoid pass the registration check

	if !ok {
		// no account in session, so just redirect to the login page, then the user could restart the process
		ctx.Redirect(setting.AppSubURL + "/user/login")
		return
	}

	if missingFields, ok := gothUser.RawData["__giteaAutoRegMissingFields"].([]string); ok {
		ctx.Data["AutoRegistrationFailedPrompt"] = ctx.Tr("auth.oauth_callback_unable_auto_reg", gothUser.Provider, strings.Join(missingFields, ","))
	}

	uname, err := extractUserNameFromOAuth2(&gothUser)
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}
	email := gothUser.Email
	ctx.Data["user_name"] = uname
	ctx.Data["email"] = email

	if email != "" {
		u, err := user_model.GetUserByEmail(ctx, email)
		if err != nil && !user_model.IsErrUserNotExist(err) {
			ctx.ServerError("UserSignIn", err)
			return
		}
		if u != nil {
			ctx.Data["user_exists"] = true
		}
	} else if uname != "" {
		u, err := user_model.GetUserByName(ctx, uname)
		if err != nil && !user_model.IsErrUserNotExist(err) {
			ctx.ServerError("UserSignIn", err)
			return
		}
		if u != nil {
			ctx.Data["user_exists"] = true
		}
	}

	ctx.HTML(http.StatusOK, tplLinkAccount)
}

func handleSignInError(ctx *context.Context, userName string, ptrForm any, tmpl templates.TplName, invoker string, err error) {
	if errors.Is(err, util.ErrNotExist) {
		ctx.RenderWithErr(ctx.Tr("form.username_password_incorrect"), tmpl, ptrForm)
	} else if errors.Is(err, util.ErrInvalidArgument) {
		ctx.Data["user_exists"] = true
		ctx.RenderWithErr(ctx.Tr("form.username_password_incorrect"), tmpl, ptrForm)
	} else if user_model.IsErrUserProhibitLogin(err) {
		ctx.Data["user_exists"] = true
		log.Info("Failed authentication attempt for %s from %s: %v", userName, ctx.RemoteAddr(), err)
		ctx.Data["Title"] = ctx.Tr("auth.prohibit_login")
		ctx.HTML(http.StatusOK, "user/auth/prohibit_login")
	} else if user_model.IsErrUserInactive(err) {
		ctx.Data["user_exists"] = true
		if setting.Service.RegisterEmailConfirm {
			ctx.Data["Title"] = ctx.Tr("auth.active_your_account")
			ctx.HTML(http.StatusOK, TplActivate)
		} else {
			log.Info("Failed authentication attempt for %s from %s: %v", userName, ctx.RemoteAddr(), err)
			ctx.Data["Title"] = ctx.Tr("auth.prohibit_login")
			ctx.HTML(http.StatusOK, "user/auth/prohibit_login")
		}
	} else {
		ctx.ServerError(invoker, err)
	}
}

// LinkAccountPostSignIn handle the coupling of external account with another account using signIn
func LinkAccountPostSignIn(ctx *context.Context) {
	signInForm := web.GetForm(ctx).(*forms.SignInForm)
	ctx.Data["DisablePassword"] = !setting.Service.RequireExternalRegistrationPassword || setting.Service.AllowOnlyExternalRegistration
	ctx.Data["Title"] = ctx.Tr("link_account")
	ctx.Data["LinkAccountMode"] = true
	ctx.Data["LinkAccountModeSignIn"] = true
	ctx.Data["EnableCaptcha"] = setting.Service.EnableCaptcha && setting.Service.RequireExternalRegistrationCaptcha
	ctx.Data["RecaptchaURL"] = setting.Service.RecaptchaURL
	ctx.Data["Captcha"] = context.GetImageCaptcha()
	ctx.Data["CaptchaType"] = setting.Service.CaptchaType
	ctx.Data["RecaptchaSitekey"] = setting.Service.RecaptchaSitekey
	ctx.Data["HcaptchaSitekey"] = setting.Service.HcaptchaSitekey
	ctx.Data["McaptchaSitekey"] = setting.Service.McaptchaSitekey
	ctx.Data["McaptchaURL"] = setting.Service.McaptchaURL
	ctx.Data["CfTurnstileSitekey"] = setting.Service.CfTurnstileSitekey
	ctx.Data["DisableRegistration"] = setting.Service.DisableRegistration
	ctx.Data["AllowOnlyInternalRegistration"] = setting.Service.AllowOnlyInternalRegistration
	ctx.Data["EnablePasswordSignInForm"] = setting.Service.EnablePasswordSignInForm
	ctx.Data["ShowRegistrationButton"] = false
	ctx.Data["EnablePasskeyAuth"] = setting.Service.EnablePasskeyAuth

	// use this to set the right link into the signIn and signUp templates in the link_account template
	ctx.Data["SignInLink"] = setting.AppSubURL + "/user/link_account_signin"
	ctx.Data["SignUpLink"] = setting.AppSubURL + "/user/link_account_signup"

	gothUser := ctx.Session.Get("linkAccountGothUser")
	if gothUser == nil {
		ctx.ServerError("UserSignIn", errors.New("not in LinkAccount session"))
		return
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplLinkAccount)
		return
	}

	u, _, err := auth_service.UserSignIn(ctx, signInForm.UserName, signInForm.Password)
	if err != nil {
		handleSignInError(ctx, signInForm.UserName, &signInForm, tplLinkAccount, "UserLinkAccount", err)
		return
	}

	linkAccount(ctx, u, gothUser.(goth.User), signInForm.Remember)
}

func linkAccount(ctx *context.Context, u *user_model.User, gothUser goth.User, remember bool) {
	updateAvatarIfNeed(ctx, gothUser.AvatarURL, u)

	// If this user is enrolled in 2FA, we can't sign the user in just yet.
	// Instead, redirect them to the 2FA authentication page.
	// We deliberately ignore the skip local 2fa setting here because we are linking to a previous user here
	_, err := auth.GetTwoFactorByUID(ctx, u.ID)
	if err != nil {
		if !auth.IsErrTwoFactorNotEnrolled(err) {
			ctx.ServerError("UserLinkAccount", err)
			return
		}

		err = externalaccount.LinkAccountToUser(ctx, u, gothUser)
		if err != nil {
			ctx.ServerError("UserLinkAccount", err)
			return
		}

		handleSignIn(ctx, u, remember)
		return
	}

	if err := updateSession(ctx, nil, map[string]any{
		// User needs to use 2FA, save data and redirect to 2FA page.
		"twofaUid":      u.ID,
		"twofaRemember": remember,
		"linkAccount":   true,
	}); err != nil {
		ctx.ServerError("RegenerateSession", err)
		return
	}

	// If WebAuthn is enrolled -> Redirect to WebAuthn instead
	regs, err := auth.GetWebAuthnCredentialsByUID(ctx, u.ID)
	if err == nil && len(regs) > 0 {
		ctx.Redirect(setting.AppSubURL + "/user/webauthn")
		return
	}

	ctx.Redirect(setting.AppSubURL + "/user/two_factor")
}

// LinkAccountPostRegister handle the creation of a new account for an external account using signUp
func LinkAccountPostRegister(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.RegisterForm)
	// TODO Make insecure passwords optional for local accounts also,
	//      once email-based Second-Factor Auth is available
	ctx.Data["DisablePassword"] = !setting.Service.RequireExternalRegistrationPassword || setting.Service.AllowOnlyExternalRegistration
	ctx.Data["Title"] = ctx.Tr("link_account")
	ctx.Data["LinkAccountMode"] = true
	ctx.Data["LinkAccountModeRegister"] = true
	ctx.Data["EnableCaptcha"] = setting.Service.EnableCaptcha && setting.Service.RequireExternalRegistrationCaptcha
	ctx.Data["RecaptchaURL"] = setting.Service.RecaptchaURL
	ctx.Data["Captcha"] = context.GetImageCaptcha()
	ctx.Data["CaptchaType"] = setting.Service.CaptchaType
	ctx.Data["RecaptchaSitekey"] = setting.Service.RecaptchaSitekey
	ctx.Data["HcaptchaSitekey"] = setting.Service.HcaptchaSitekey
	ctx.Data["McaptchaSitekey"] = setting.Service.McaptchaSitekey
	ctx.Data["McaptchaURL"] = setting.Service.McaptchaURL
	ctx.Data["CfTurnstileSitekey"] = setting.Service.CfTurnstileSitekey
	ctx.Data["DisableRegistration"] = setting.Service.DisableRegistration
	ctx.Data["AllowOnlyInternalRegistration"] = setting.Service.AllowOnlyInternalRegistration
	ctx.Data["EnablePasswordSignInForm"] = setting.Service.EnablePasswordSignInForm
	ctx.Data["ShowRegistrationButton"] = false
	ctx.Data["EnablePasskeyAuth"] = setting.Service.EnablePasskeyAuth

	// use this to set the right link into the signIn and signUp templates in the link_account template
	ctx.Data["SignInLink"] = setting.AppSubURL + "/user/link_account_signin"
	ctx.Data["SignUpLink"] = setting.AppSubURL + "/user/link_account_signup"

	gothUserInterface := ctx.Session.Get("linkAccountGothUser")
	if gothUserInterface == nil {
		ctx.ServerError("UserSignUp", errors.New("not in LinkAccount session"))
		return
	}
	gothUser, ok := gothUserInterface.(goth.User)
	if !ok {
		ctx.ServerError("UserSignUp", fmt.Errorf("session linkAccountGothUser type is %t but not goth.User", gothUserInterface))
		return
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplLinkAccount)
		return
	}

	if setting.Service.DisableRegistration || setting.Service.AllowOnlyInternalRegistration {
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	if setting.Service.EnableCaptcha && setting.Service.RequireExternalRegistrationCaptcha {
		context.VerifyCaptcha(ctx, tplLinkAccount, form)
		if ctx.Written() {
			return
		}
	}

	if !form.IsEmailDomainAllowed() {
		ctx.RenderWithErr(ctx.Tr("auth.email_domain_blacklisted"), tplLinkAccount, &form)
		return
	}

	if setting.Service.AllowOnlyExternalRegistration || !setting.Service.RequireExternalRegistrationPassword {
		// In user_model.User an empty password is classed as not set, so we set form.Password to empty.
		// Eventually the database should be changed to indicate "Second Factor"-enabled accounts
		// (accounts that do not introduce the security vulnerabilities of a password).
		// If a user decides to circumvent second-factor security, and purposefully create a password,
		// they can still do so using the "Recover Account" option.
		form.Password = ""
	} else {
		if (len(strings.TrimSpace(form.Password)) > 0 || len(strings.TrimSpace(form.Retype)) > 0) && form.Password != form.Retype {
			ctx.Data["Err_Password"] = true
			ctx.RenderWithErr(ctx.Tr("form.password_not_match"), tplLinkAccount, &form)
			return
		}
		if len(strings.TrimSpace(form.Password)) > 0 && len(form.Password) < setting.MinPasswordLength {
			ctx.Data["Err_Password"] = true
			ctx.RenderWithErr(ctx.Tr("auth.password_too_short", setting.MinPasswordLength), tplLinkAccount, &form)
			return
		}
	}

	authSource, err := auth.GetActiveOAuth2SourceByName(ctx, gothUser.Provider)
	if err != nil {
		ctx.ServerError("CreateUser", err)
		return
	}

	u := &user_model.User{
		Name:        form.UserName,
		Email:       form.Email,
		Passwd:      form.Password,
		LoginType:   auth.OAuth2,
		LoginSource: authSource.ID,
		LoginName:   gothUser.UserID,
	}

	if !createAndHandleCreatedUser(ctx, tplLinkAccount, form, u, nil, &gothUser, false) {
		// error already handled
		return
	}

	source := authSource.Cfg.(*oauth2.Source)
	if err := syncGroupsToTeams(ctx, source, &gothUser, u); err != nil {
		ctx.ServerError("SyncGroupsToTeams", err)
		return
	}

	handleSignIn(ctx, u, false)
}
