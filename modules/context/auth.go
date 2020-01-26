// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"gitea.com/macaron/csrf"
	"gitea.com/macaron/macaron"
)

// ToggleOptions contains required or check options
type ToggleOptions struct {
	SignInRequired  bool
	SignOutRequired bool
	AdminRequired   bool
	DisableCSRF     bool
}

// Toggle returns toggle options as middleware
func Toggle(options *ToggleOptions) macaron.Handler {
	return func(ctx *Context) {
		// Cannot view any page before installation.
		if !setting.InstallLock {
			ctx.Redirect(setting.AppSubURL + "/install")
			return
		}

		// Check prohibit login users.
		if ctx.IsSigned {
			if !ctx.User.IsActive && setting.Service.RegisterEmailConfirm {
				ctx.Data["Title"] = ctx.Tr("auth.active_your_account")
				ctx.HTML(200, "user/auth/activate")
				return
			} else if !ctx.User.IsActive || ctx.User.ProhibitLogin {
				log.Info("Failed authentication attempt for %s from %s", ctx.User.Name, ctx.RemoteAddr())
				ctx.Data["Title"] = ctx.Tr("auth.prohibit_login")
				ctx.HTML(200, "user/auth/prohibit_login")
				return
			}

			if ctx.User.MustChangePassword {
				if ctx.Req.URL.Path != "/user/settings/change_password" {
					ctx.Data["Title"] = ctx.Tr("auth.must_change_password")
					ctx.Data["ChangePasscodeLink"] = setting.AppSubURL + "/user/change_password"
					ctx.SetCookie("redirect_to", setting.AppSubURL+ctx.Req.URL.RequestURI(), 0, setting.AppSubURL)
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

		if !options.SignOutRequired && !options.DisableCSRF && ctx.Req.Method == "POST" && !auth.IsAPIPath(ctx.Req.URL.Path) {
			csrf.Validate(ctx.Context, ctx.csrf)
			if ctx.Written() {
				return
			}
		}

		if options.SignInRequired {
			if !ctx.IsSigned {
				// Restrict API calls with error message.
				if auth.IsAPIPath(ctx.Req.URL.Path) {
					ctx.JSON(403, map[string]string{
						"message": "Only signed in user is allowed to call APIs.",
					})
					return
				}

				ctx.SetCookie("redirect_to", setting.AppSubURL+ctx.Req.URL.RequestURI(), 0, setting.AppSubURL)
				ctx.Redirect(setting.AppSubURL + "/user/login")
				return
			} else if !ctx.User.IsActive && setting.Service.RegisterEmailConfirm {
				ctx.Data["Title"] = ctx.Tr("auth.active_your_account")
				ctx.HTML(200, "user/auth/activate")
				return
			}
			if ctx.IsSigned && auth.IsAPIPath(ctx.Req.URL.Path) && ctx.IsBasicAuth {
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
		if !options.SignOutRequired && !ctx.IsSigned && !auth.IsAPIPath(ctx.Req.URL.Path) &&
			len(ctx.GetCookie(setting.CookieUserName)) > 0 {
			ctx.SetCookie("redirect_to", setting.AppSubURL+ctx.Req.URL.RequestURI(), 0, setting.AppSubURL)
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
