// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web/middleware"
)

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
		// Check prohibit login users.
		if ctx.IsSigned {
			if !ctx.User.IsActive && setting.Service.RegisterEmailConfirm {
				ctx.Data["Title"] = ctx.Tr("auth.active_your_account")
				ctx.HTML(http.StatusOK, "user/auth/activate")
				return
			}
			if !ctx.User.IsActive || ctx.User.ProhibitLogin {
				log.Info("Failed authentication attempt for %s from %s", ctx.User.Name, ctx.RemoteAddr())
				ctx.Data["Title"] = ctx.Tr("auth.prohibit_login")
				ctx.HTML(http.StatusOK, "user/auth/prohibit_login")
				return
			}

			if ctx.User.MustChangePassword {
				if ctx.Req.URL.Path != "/user/settings/change_password" {
					ctx.Data["Title"] = ctx.Tr("auth.must_change_password")
					ctx.Data["ChangePasscodeLink"] = setting.AppSubURL + "/user/change_password"
					if ctx.Req.URL.Path != "/user/events" {
						middleware.SetRedirectToCookie(ctx.Resp, setting.AppSubURL+ctx.Req.URL.RequestURI())
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

		if !options.SignOutRequired && !options.DisableCSRF && ctx.Req.Method == "POST" {
			Validate(ctx, ctx.csrf)
			if ctx.Written() {
				return
			}
		}

		if options.SignInRequired {
			if !ctx.IsSigned {
				if ctx.Req.URL.Path != "/user/events" {
					middleware.SetRedirectToCookie(ctx.Resp, setting.AppSubURL+ctx.Req.URL.RequestURI())
				}
				ctx.Redirect(setting.AppSubURL + "/user/login")
				return
			} else if !ctx.User.IsActive && setting.Service.RegisterEmailConfirm {
				ctx.Data["Title"] = ctx.Tr("auth.active_your_account")
				ctx.HTML(http.StatusOK, "user/auth/activate")
				return
			}
		}

		// Redirect to log in page if auto-signin info is provided and has not signed in.
		if !options.SignOutRequired && !ctx.IsSigned &&
			len(ctx.GetCookie(setting.CookieUserName)) > 0 {
			if ctx.Req.URL.Path != "/user/events" {
				middleware.SetRedirectToCookie(ctx.Resp, setting.AppSubURL+ctx.Req.URL.RequestURI())
			}
			ctx.Redirect(setting.AppSubURL + "/user/login")
			return
		}

		if options.AdminRequired {
			if !ctx.User.IsAdmin {
				ctx.Error(http.StatusForbidden)
				return
			}
			ctx.Data["PageIsAdmin"] = true
		}
	}
}

// ToggleAPI returns toggle options as middleware
func ToggleAPI(options *ToggleOptions) func(ctx *APIContext) {
	return func(ctx *APIContext) {
		// Check prohibit login users.
		if ctx.IsSigned {
			if !ctx.User.IsActive && setting.Service.RegisterEmailConfirm {
				ctx.Data["Title"] = ctx.Tr("auth.active_your_account")
				ctx.JSON(http.StatusForbidden, map[string]string{
					"message": "This account is not activated.",
				})
				return
			}
			if !ctx.User.IsActive || ctx.User.ProhibitLogin {
				log.Info("Failed authentication attempt for %s from %s", ctx.User.Name, ctx.RemoteAddr())
				ctx.Data["Title"] = ctx.Tr("auth.prohibit_login")
				ctx.JSON(http.StatusForbidden, map[string]string{
					"message": "This account is prohibited from signing in, please contact your site administrator.",
				})
				return
			}

			if ctx.User.MustChangePassword {
				ctx.JSON(http.StatusForbidden, map[string]string{
					"message": "You must change your password. Change it at: " + setting.AppURL + "/user/change_password",
				})
				return
			}
		}

		// Redirect to dashboard if user tries to visit any non-login page.
		if options.SignOutRequired && ctx.IsSigned && ctx.Req.URL.RequestURI() != "/" {
			ctx.Redirect(setting.AppSubURL + "/")
			return
		}

		if options.SignInRequired {
			if !ctx.IsSigned {
				// Restrict API calls with error message.
				ctx.JSON(http.StatusForbidden, map[string]string{
					"message": "Only signed in user is allowed to call APIs.",
				})
				return
			} else if !ctx.User.IsActive && setting.Service.RegisterEmailConfirm {
				ctx.Data["Title"] = ctx.Tr("auth.active_your_account")
				ctx.HTML(http.StatusOK, "user/auth/activate")
				return
			}
			if ctx.IsSigned && ctx.IsBasicAuth {
				twofa, err := models.GetTwoFactorByUID(ctx.User.ID)
				if err != nil {
					if models.IsErrTwoFactorNotEnrolled(err) {
						return // No 2FA enrollment for this user
					}
					ctx.InternalServerError(err)
					return
				}
				otpHeader := ctx.Req.Header.Get("X-Gitea-OTP")
				ok, err := twofa.ValidateTOTP(otpHeader)
				if err != nil {
					ctx.InternalServerError(err)
					return
				}
				if !ok {
					ctx.JSON(http.StatusForbidden, map[string]string{
						"message": "Only signed in user is allowed to call APIs.",
					})
					return
				}
			}
		}

		if options.AdminRequired {
			if !ctx.User.IsAdmin {
				ctx.JSON(http.StatusForbidden, map[string]string{
					"message": "You have no permission to request for this.",
				})
				return
			}
			ctx.Data["PageIsAdmin"] = true
		}
	}
}
