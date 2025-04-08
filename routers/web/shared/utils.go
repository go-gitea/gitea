// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package shared

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/routers/common"
	"code.gitea.io/gitea/services/context"
)

func SitemapEnabled(ctx *context.Context) {
	if !setting.Other.EnableSitemap {
		ctx.HTTPError(http.StatusNotFound)
		return
	}
}

// verifyAuthWithOptions checks authentication according to options
func verifyAuthWithOptions(options *common.VerifyOptions) func(ctx *context.Context) {
	return func(ctx *context.Context) {
		// Check prohibit login users.
		if ctx.IsSigned {
			if !ctx.Doer.IsActive && setting.Service.RegisterEmailConfirm {
				ctx.Data["Title"] = ctx.Tr("auth.active_your_account")
				ctx.HTML(http.StatusOK, "user/auth/activate")
				return
			}
			if !ctx.Doer.IsActive || ctx.Doer.ProhibitLogin {
				log.Info("Failed authentication attempt for %s from %s", ctx.Doer.Name, ctx.RemoteAddr())
				ctx.Data["Title"] = ctx.Tr("auth.prohibit_login")
				ctx.HTML(http.StatusOK, "user/auth/prohibit_login")
				return
			}

			if ctx.Doer.MustChangePassword {
				if ctx.Req.URL.Path != "/user/settings/change_password" {
					if strings.HasPrefix(ctx.Req.UserAgent(), "git") {
						ctx.HTTPError(http.StatusUnauthorized, ctx.Locale.TrString("auth.must_change_password"))
						return
					}
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

		// Redirect to dashboard (or alternate location) if user tries to visit any non-login page.
		if options.SignOutRequired && ctx.IsSigned && ctx.Req.URL.RequestURI() != "/" {
			ctx.RedirectToCurrentSite(ctx.FormString("redirect_to"))
			return
		}

		if !options.SignOutRequired && !options.DisableCSRF && ctx.Req.Method == http.MethodPost {
			ctx.Csrf.Validate(ctx)
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
			} else if !ctx.Doer.IsActive && setting.Service.RegisterEmailConfirm {
				ctx.Data["Title"] = ctx.Tr("auth.active_your_account")
				ctx.HTML(http.StatusOK, "user/auth/activate")
				return
			}
		}

		// Redirect to log in page if auto-signin info is provided and has not signed in.
		if !options.SignOutRequired && !ctx.IsSigned &&
			ctx.GetSiteCookie(setting.CookieRememberName) != "" {
			if ctx.Req.URL.Path != "/user/events" {
				middleware.SetRedirectToCookie(ctx.Resp, setting.AppSubURL+ctx.Req.URL.RequestURI())
			}
			ctx.Redirect(setting.AppSubURL + "/user/login")
			return
		}

		if options.AdminRequired {
			if !ctx.Doer.IsAdmin {
				ctx.HTTPError(http.StatusForbidden)
				return
			}
			ctx.Data["PageIsAdmin"] = true
		}
	}
}

var (
	OptSignInIgnoreCsrf = verifyAuthWithOptions(&common.VerifyOptions{DisableCSRF: true})

	// required to be signed in or signed out
	ReqSignIn  = verifyAuthWithOptions(&common.VerifyOptions{SignInRequired: true})
	ReqSignOut = verifyAuthWithOptions(&common.VerifyOptions{SignOutRequired: true})
	// optional sign in (if signed in, use the user as doer, if not, no doer)
	OptSignIn        = verifyAuthWithOptions(&common.VerifyOptions{SignInRequired: setting.Service.RequireSignInViewStrict})
	OptExploreSignIn = verifyAuthWithOptions(&common.VerifyOptions{SignInRequired: setting.Service.RequireSignInViewStrict || setting.Service.Explore.RequireSigninView})

	AdminReq = verifyAuthWithOptions(&common.VerifyOptions{SignInRequired: true, AdminRequired: true})
)

func OpenIDSignUpEnabled(ctx *context.Context) {
	if !setting.Service.EnableOpenIDSignUp {
		ctx.HTTPError(http.StatusForbidden)
		return
	}
}

func OpenIDSignInEnabled(ctx *context.Context) {
	if !setting.Service.EnableOpenIDSignIn {
		ctx.HTTPError(http.StatusForbidden)
		return
	}
}

func LinkAccountEnabled(ctx *context.Context) {
	if !setting.Service.EnableOpenIDSignIn && !setting.Service.EnableOpenIDSignUp && !setting.OAuth2.Enabled {
		ctx.HTTPError(http.StatusForbidden)
		return
	}
}

func Oauth2Enabled(ctx *context.Context) {
	if !setting.OAuth2.Enabled {
		ctx.HTTPError(http.StatusForbidden)
		return
	}
}

func PackagesEnabled(ctx *context.Context) {
	if !setting.Packages.Enabled {
		ctx.HTTPError(http.StatusForbidden)
		return
	}
}

// WebhooksEnabled requires webhooks to be enabled by admin.
func WebhooksEnabled(ctx *context.Context) {
	if setting.DisableWebhooks {
		ctx.HTTPError(http.StatusForbidden)
		return
	}
}
