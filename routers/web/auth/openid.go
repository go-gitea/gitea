// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"fmt"
	"net/http"
	"net/url"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/auth/openid"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
)

const (
	tplSignInOpenID templates.TplName = "user/auth/signin_openid"
	tplConnectOID   templates.TplName = "user/auth/signup_openid_connect"
	tplSignUpOID    templates.TplName = "user/auth/signup_openid_register"
)

// SignInOpenID render sign in page
func SignInOpenID(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("sign_in")

	if ctx.FormString("openid.return_to") != "" {
		signInOpenIDVerify(ctx)
		return
	}

	if CheckAutoLogin(ctx) {
		return
	}

	ctx.Data["PageIsSignIn"] = true
	ctx.Data["PageIsLoginOpenID"] = true
	ctx.HTML(http.StatusOK, tplSignInOpenID)
}

// Check if the given OpenID URI is allowed by blacklist/whitelist
func allowedOpenIDURI(uri string) (err error) {
	// In case a Whitelist is present, URI must be in it
	// in order to be accepted
	if len(setting.Service.OpenIDWhitelist) != 0 {
		for _, pat := range setting.Service.OpenIDWhitelist {
			if pat.MatchString(uri) {
				return nil // pass
			}
		}
		// must match one of this or be refused
		return fmt.Errorf("URI not allowed by whitelist")
	}

	// A blacklist match expliclty forbids
	for _, pat := range setting.Service.OpenIDBlacklist {
		if pat.MatchString(uri) {
			return fmt.Errorf("URI forbidden by blacklist")
		}
	}

	return nil
}

// SignInOpenIDPost response for openid sign in request
func SignInOpenIDPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.SignInOpenIDForm)
	ctx.Data["Title"] = ctx.Tr("sign_in")
	ctx.Data["PageIsSignIn"] = true
	ctx.Data["PageIsLoginOpenID"] = true

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplSignInOpenID)
		return
	}

	id, err := openid.Normalize(form.Openid)
	if err != nil {
		ctx.RenderWithErr(err.Error(), tplSignInOpenID, &form)
		return
	}
	form.Openid = id

	log.Trace("OpenID uri: " + id)

	err = allowedOpenIDURI(id)
	if err != nil {
		ctx.RenderWithErr(err.Error(), tplSignInOpenID, &form)
		return
	}

	redirectTo := setting.AppURL + "user/login/openid"
	url, err := openid.RedirectURL(id, redirectTo, setting.AppURL)
	if err != nil {
		log.Error("Error in OpenID redirect URL: %s, %v", redirectTo, err.Error())
		ctx.RenderWithErr(fmt.Sprintf("Unable to find OpenID provider in %s", redirectTo), tplSignInOpenID, &form)
		return
	}

	// Request optional nickname and email info
	// NOTE: change to `openid.sreg.required` to require it
	url += "&openid.ns.sreg=http%3A%2F%2Fopenid.net%2Fextensions%2Fsreg%2F1.1"
	url += "&openid.sreg.optional=nickname%2Cemail"

	log.Trace("Form-passed openid-remember: %t", form.Remember)

	if err := ctx.Session.Set("openid_signin_remember", form.Remember); err != nil {
		log.Error("SignInOpenIDPost: Could not set openid_signin_remember in session: %v", err)
	}
	if err := ctx.Session.Release(); err != nil {
		log.Error("SignInOpenIDPost: Unable to save changes to the session: %v", err)
	}

	ctx.Redirect(url)
}

// signInOpenIDVerify handles response from OpenID provider
func signInOpenIDVerify(ctx *context.Context) {
	log.Trace("Incoming call to: %s", ctx.Req.URL.String())

	fullURL := setting.AppURL + ctx.Req.URL.String()[1:]
	log.Trace("Full URL: %s", fullURL)

	id, err := openid.Verify(fullURL)
	if err != nil {
		ctx.RenderWithErr(err.Error(), tplSignInOpenID, &forms.SignInOpenIDForm{
			Openid: id,
		})
		return
	}

	log.Trace("Verified ID: %s", id)

	/* Now we should seek for the user and log him in, or prompt
	 * to register if not found */

	u, err := user_model.GetUserByOpenID(ctx, id)
	if err != nil {
		if !user_model.IsErrUserNotExist(err) {
			ctx.RenderWithErr(err.Error(), tplSignInOpenID, &forms.SignInOpenIDForm{
				Openid: id,
			})
			return
		}
		log.Error("signInOpenIDVerify: %v", err)
	}
	if u != nil {
		log.Trace("User exists, logging in")
		remember, _ := ctx.Session.Get("openid_signin_remember").(bool)
		log.Trace("Session stored openid-remember: %t", remember)
		handleSignIn(ctx, u, remember)
		return
	}

	log.Trace("User with openid: %s does not exist, should connect or register", id)

	parsedURL, err := url.Parse(fullURL)
	if err != nil {
		ctx.RenderWithErr(err.Error(), tplSignInOpenID, &forms.SignInOpenIDForm{
			Openid: id,
		})
		return
	}
	values, err := url.ParseQuery(parsedURL.RawQuery)
	if err != nil {
		ctx.RenderWithErr(err.Error(), tplSignInOpenID, &forms.SignInOpenIDForm{
			Openid: id,
		})
		return
	}
	email := values.Get("openid.sreg.email")
	nickname := values.Get("openid.sreg.nickname")

	log.Trace("User has email=%s and nickname=%s", email, nickname)

	if email != "" {
		u, err = user_model.GetUserByEmail(ctx, email)
		if err != nil {
			if !user_model.IsErrUserNotExist(err) {
				ctx.RenderWithErr(err.Error(), tplSignInOpenID, &forms.SignInOpenIDForm{
					Openid: id,
				})
				return
			}
			log.Error("signInOpenIDVerify: %v", err)
		}
		if u != nil {
			log.Trace("Local user %s has OpenID provided email %s", u.LowerName, email)
		}
	}

	if u == nil && nickname != "" {
		u, _ = user_model.GetUserByName(ctx, nickname)
		if err != nil {
			if !user_model.IsErrUserNotExist(err) {
				ctx.RenderWithErr(err.Error(), tplSignInOpenID, &forms.SignInOpenIDForm{
					Openid: id,
				})
				return
			}
		}
		if u != nil {
			log.Trace("Local user %s has OpenID provided nickname %s", u.LowerName, nickname)
		}
	}

	if u != nil {
		nickname = u.LowerName
	}
	if err := updateSession(ctx, nil, map[string]any{
		"openid_verified_uri":        id,
		"openid_determined_email":    email,
		"openid_determined_username": nickname,
	}); err != nil {
		ctx.ServerError("updateSession", err)
		return
	}

	if u != nil || !setting.Service.EnableOpenIDSignUp || setting.Service.AllowOnlyInternalRegistration {
		ctx.Redirect(setting.AppSubURL + "/user/openid/connect")
	} else {
		ctx.Redirect(setting.AppSubURL + "/user/openid/register")
	}
}

// ConnectOpenID shows a form to connect an OpenID URI to an existing account
func ConnectOpenID(ctx *context.Context) {
	oid, _ := ctx.Session.Get("openid_verified_uri").(string)
	if oid == "" {
		ctx.Redirect(setting.AppSubURL + "/user/login/openid")
		return
	}
	ctx.Data["Title"] = "OpenID connect"
	ctx.Data["PageIsSignIn"] = true
	ctx.Data["PageIsOpenIDConnect"] = true
	ctx.Data["EnableOpenIDSignUp"] = setting.Service.EnableOpenIDSignUp
	ctx.Data["AllowOnlyInternalRegistration"] = setting.Service.AllowOnlyInternalRegistration
	ctx.Data["OpenID"] = oid
	userName, _ := ctx.Session.Get("openid_determined_username").(string)
	if userName != "" {
		ctx.Data["user_name"] = userName
	}
	ctx.HTML(http.StatusOK, tplConnectOID)
}

// ConnectOpenIDPost handles submission of a form to connect an OpenID URI to an existing account
func ConnectOpenIDPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.ConnectOpenIDForm)
	oid, _ := ctx.Session.Get("openid_verified_uri").(string)
	if oid == "" {
		ctx.Redirect(setting.AppSubURL + "/user/login/openid")
		return
	}
	ctx.Data["Title"] = "OpenID connect"
	ctx.Data["PageIsSignIn"] = true
	ctx.Data["PageIsOpenIDConnect"] = true
	ctx.Data["EnableOpenIDSignUp"] = setting.Service.EnableOpenIDSignUp
	ctx.Data["OpenID"] = oid

	u, _, err := auth.UserSignIn(ctx, form.UserName, form.Password)
	if err != nil {
		handleSignInError(ctx, form.UserName, &form, tplConnectOID, "ConnectOpenIDPost", err)
		return
	}

	// add OpenID for the user
	userOID := &user_model.UserOpenID{UID: u.ID, URI: oid}
	if err = user_model.AddUserOpenID(ctx, userOID); err != nil {
		if user_model.IsErrOpenIDAlreadyUsed(err) {
			ctx.RenderWithErr(ctx.Tr("form.openid_been_used", oid), tplConnectOID, &form)
			return
		}
		ctx.ServerError("AddUserOpenID", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("settings.add_openid_success"))

	remember, _ := ctx.Session.Get("openid_signin_remember").(bool)
	log.Trace("Session stored openid-remember: %t", remember)
	handleSignIn(ctx, u, remember)
}

// RegisterOpenID shows a form to create a new user authenticated via an OpenID URI
func RegisterOpenID(ctx *context.Context) {
	oid, _ := ctx.Session.Get("openid_verified_uri").(string)
	if oid == "" {
		ctx.Redirect(setting.AppSubURL + "/user/login/openid")
		return
	}
	ctx.Data["Title"] = "OpenID signup"
	ctx.Data["PageIsSignIn"] = true
	ctx.Data["PageIsOpenIDRegister"] = true
	ctx.Data["EnableOpenIDSignUp"] = setting.Service.EnableOpenIDSignUp
	ctx.Data["AllowOnlyInternalRegistration"] = setting.Service.AllowOnlyInternalRegistration
	ctx.Data["EnableCaptcha"] = setting.Service.EnableCaptcha
	ctx.Data["Captcha"] = context.GetImageCaptcha()
	ctx.Data["CaptchaType"] = setting.Service.CaptchaType
	ctx.Data["RecaptchaSitekey"] = setting.Service.RecaptchaSitekey
	ctx.Data["HcaptchaSitekey"] = setting.Service.HcaptchaSitekey
	ctx.Data["RecaptchaURL"] = setting.Service.RecaptchaURL
	ctx.Data["McaptchaSitekey"] = setting.Service.McaptchaSitekey
	ctx.Data["McaptchaURL"] = setting.Service.McaptchaURL
	ctx.Data["CfTurnstileSitekey"] = setting.Service.CfTurnstileSitekey
	ctx.Data["OpenID"] = oid
	userName, _ := ctx.Session.Get("openid_determined_username").(string)
	if userName != "" {
		ctx.Data["user_name"] = userName
	}
	email, _ := ctx.Session.Get("openid_determined_email").(string)
	if email != "" {
		ctx.Data["email"] = email
	}
	ctx.HTML(http.StatusOK, tplSignUpOID)
}

// RegisterOpenIDPost handles submission of a form to create a new user authenticated via an OpenID URI
func RegisterOpenIDPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.SignUpOpenIDForm)
	oid, _ := ctx.Session.Get("openid_verified_uri").(string)
	if oid == "" {
		ctx.Redirect(setting.AppSubURL + "/user/login/openid")
		return
	}

	ctx.Data["Title"] = "OpenID signup"
	ctx.Data["PageIsSignIn"] = true
	ctx.Data["PageIsOpenIDRegister"] = true
	ctx.Data["EnableOpenIDSignUp"] = setting.Service.EnableOpenIDSignUp
	context.SetCaptchaData(ctx)
	ctx.Data["OpenID"] = oid

	if setting.Service.AllowOnlyInternalRegistration {
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	if setting.Service.EnableCaptcha {
		if err := ctx.Req.ParseForm(); err != nil {
			ctx.ServerError("", err)
			return
		}
		context.VerifyCaptcha(ctx, tplSignUpOID, form)
	}

	length := setting.MinPasswordLength
	if length < 256 {
		length = 256
	}
	password, err := util.CryptoRandomString(int64(length))
	if err != nil {
		ctx.RenderWithErr(err.Error(), tplSignUpOID, form)
		return
	}

	u := &user_model.User{
		Name:   form.UserName,
		Email:  form.Email,
		Passwd: password,
	}
	if !createUserInContext(ctx, tplSignUpOID, form, u, nil, nil, false) {
		// error already handled
		return
	}

	// add OpenID for the user
	userOID := &user_model.UserOpenID{UID: u.ID, URI: oid}
	if err = user_model.AddUserOpenID(ctx, userOID); err != nil {
		if user_model.IsErrOpenIDAlreadyUsed(err) {
			ctx.RenderWithErr(ctx.Tr("form.openid_been_used", oid), tplSignUpOID, &form)
			return
		}
		ctx.ServerError("AddUserOpenID", err)
		return
	}

	if !handleUserCreated(ctx, u, nil) {
		// error already handled
		return
	}

	remember, _ := ctx.Session.Get("openid_signin_remember").(bool)
	log.Trace("Session stored openid-remember: %t", remember)
	handleSignIn(ctx, u, remember)
}
