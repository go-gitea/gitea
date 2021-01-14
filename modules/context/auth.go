// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// IsAPIPath if URL is an api path
func IsAPIPath(url string) bool {
	return strings.HasPrefix(url, "/api/")
}

// ToggleOptions contains required or check options
type ToggleOptions struct {
	SignInRequired  bool
	SignOutRequired bool
	AdminRequired   bool
	DisableCSRF     bool
}

// Toggle returns toggle options as middleware
func Toggle(options *ToggleOptions) func(ctx *Context) {
	return func(ctx *Context) {
		isAPIPath := IsAPIPath(ctx.Req.URL.Path)

		// Check prohibit login users.
		if ctx.IsSigned {
			if !ctx.User.IsActive && setting.Service.RegisterEmailConfirm {
				ctx.Data["Title"] = ctx.Tr("auth.active_your_account")
				if isAPIPath {
					ctx.JSON(403, map[string]string{
						"message": "This account is not activated.",
					})
					return
				}
				ctx.HTML(200, "user/auth/activate")
				return
			} else if !ctx.User.IsActive || ctx.User.ProhibitLogin {
				log.Info("Failed authentication attempt for %s from %s", ctx.User.Name, ctx.RemoteAddr())
				ctx.Data["Title"] = ctx.Tr("auth.prohibit_login")
				if isAPIPath {
					ctx.JSON(403, map[string]string{
						"message": "This account is prohibited from signing in, please contact your site administrator.",
					})
					return
				}
				ctx.HTML(200, "user/auth/prohibit_login")
				return
			}

			if ctx.User.MustChangePassword {
				if isAPIPath {
					ctx.JSON(403, map[string]string{
						"message": "You must change your password. Change it at: " + setting.AppURL + "/user/change_password",
					})
					return
				}
				if ctx.Req.URL.Path != "/user/settings/change_password" {
					ctx.Data["Title"] = ctx.Tr("auth.must_change_password")
					ctx.Data["ChangePasscodeLink"] = setting.AppSubURL + "/user/change_password"
					if ctx.Req.URL.Path != "/user/events" {
						ctx.SetCookie("redirect_to", setting.AppSubURL+ctx.Req.URL.RequestURI(), 0, setting.AppSubURL)
					}
					ctx.Redirect(setting.AppSubURL + "/user/settings/change_password")
					return
				}
			} else if ctx.Req.URL.Path == "/user/settings/change_password" {
				// make sure that the form cannot be accessed by users who don't need this
				ctx.Redirect(setting.AppSubURL + "/")
				return
			}
		}

		// Redirect to dashboard if user tries to visit any non-login page.
		if options.SignOutRequired && ctx.IsSigned && ctx.Req.URL.RequestURI() != "/" {
			ctx.Redirect(setting.AppSubURL + "/")
			return
		}

		if !options.SignOutRequired && !options.DisableCSRF && ctx.Req.Method == "POST" && !IsAPIPath(ctx.Req.URL.Path) {
			Validate(ctx, ctx.csrf)
			if ctx.Written() {
				return
			}
		}

		if options.SignInRequired {
			if !ctx.IsSigned {
				// Restrict API calls with error message.
				if isAPIPath {
					ctx.JSON(403, map[string]string{
						"message": "Only signed in user is allowed to call APIs.",
					})
					return
				}
				if ctx.Req.URL.Path != "/user/events" {
					ctx.SetCookie("redirect_to", setting.AppSubURL+ctx.Req.URL.RequestURI(), 0, setting.AppSubURL)
				}
				ctx.Redirect(setting.AppSubURL + "/user/login")
				return
			} else if !ctx.User.IsActive && setting.Service.RegisterEmailConfirm {
				ctx.Data["Title"] = ctx.Tr("auth.active_your_account")
				ctx.HTML(200, "user/auth/activate")
				return
			}
			if ctx.IsSigned && isAPIPath && ctx.IsBasicAuth {
				twofa, err := models.GetTwoFactorByUID(ctx.User.ID)
				if err != nil {
					if models.IsErrTwoFactorNotEnrolled(err) {
						return // No 2FA enrollment for this user
					}
					ctx.Error(500)
					return
				}
				otpHeader := ctx.Req.Header.Get("X-Gitea-OTP")
				ok, err := twofa.ValidateTOTP(otpHeader)
				if err != nil {
					ctx.Error(500)
					return
				}
				if !ok {
					ctx.JSON(403, map[string]string{
						"message": "Only signed in user is allowed to call APIs.",
					})
					return
				}
			}
		}

		// Redirect to log in page if auto-signin info is provided and has not signed in.
		if !options.SignOutRequired && !ctx.IsSigned && !isAPIPath &&
			len(ctx.GetCookie(setting.CookieUserName)) > 0 {
			if ctx.Req.URL.Path != "/user/events" {
				ctx.SetCookie("redirect_to", setting.AppSubURL+ctx.Req.URL.RequestURI(), 0, setting.AppSubURL)
			}
			ctx.Redirect(setting.AppSubURL + "/user/login")
			return
		}

		if options.AdminRequired {
			if !ctx.User.IsAdmin {
				ctx.Error(403)
				return
			}
			ctx.Data["PageIsAdmin"] = true
		}
	}
}
