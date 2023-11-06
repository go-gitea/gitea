// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"
	"strings"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web/middleware"
)

// Auth is a middleware to authenticate a web user
func Auth(authMethod Method) func(*context.Context) {
	return func(ctx *context.Context) {
		ar, err := authShared(ctx.Base, ctx.Session, authMethod)
		if err != nil {
			log.Error("Failed to verify user: %v", err)
			ctx.Error(http.StatusUnauthorized, "Verify")
			return
		}
		ctx.Doer = ar.Doer
		ctx.IsSigned = ar.Doer != nil
		ctx.IsBasicAuth = ar.IsBasicAuth
		if ctx.Doer == nil {
			// ensure the session uid is deleted
			_ = ctx.Session.Delete("uid")
		}
	}
}

// APIAuth is a middleware to authenticate an api user
func APIAuth(authMethod Method) func(*context.APIContext) {
	return func(ctx *context.APIContext) {
		ar, err := authShared(ctx.Base, nil, authMethod)
		if err != nil {
			ctx.Error(http.StatusUnauthorized, "APIAuth", err)
			return
		}
		ctx.Doer = ar.Doer
		ctx.IsSigned = ar.Doer != nil
		ctx.IsBasicAuth = ar.IsBasicAuth
	}
}

type authResult struct {
	Doer        *user_model.User
	IsBasicAuth bool
}

func authShared(ctx *context.Base, sessionStore SessionStore, authMethod Method) (ar authResult, err error) {
	ar.Doer, err = authMethod.Verify(ctx.Req, ctx.Resp, ctx, sessionStore)
	if err != nil {
		return ar, err
	}
	if ar.Doer != nil {
		if ctx.Locale.Language() != ar.Doer.Language {
			ctx.Locale = middleware.Locale(ctx.Resp, ctx.Req)
		}
		ar.IsBasicAuth = ctx.Data["AuthedMethod"].(string) == BasicMethodName

		ctx.Data["IsSigned"] = true
		ctx.Data[middleware.ContextDataKeySignedUser] = ar.Doer
		ctx.Data["SignedUserID"] = ar.Doer.ID
		ctx.Data["IsAdmin"] = ar.Doer.IsAdmin
	} else {
		ctx.Data["SignedUserID"] = int64(0)
	}
	return ar, nil
}

// VerifyOptions contains required or check options
type VerifyOptions struct {
	SignInRequired  bool
	SignOutRequired bool
	AdminRequired   bool
	DisableCSRF     bool
}

// VerifyAuthWithOptions checks authentication according to options
func VerifyAuthWithOptions(options *VerifyOptions) func(ctx *context.Context) {
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
						ctx.Error(http.StatusUnauthorized, ctx.Tr("auth.must_change_password"))
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

		// Redirect to dashboard if user tries to visit any non-login page.
		if options.SignOutRequired && ctx.IsSigned && ctx.Req.URL.RequestURI() != "/" {
			ctx.Redirect(setting.AppSubURL + "/")
			return
		}

		if !options.SignOutRequired && !options.DisableCSRF && ctx.Req.Method == "POST" {
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
			len(ctx.GetSiteCookie(setting.CookieUserName)) > 0 {
			if ctx.Req.URL.Path != "/user/events" {
				middleware.SetRedirectToCookie(ctx.Resp, setting.AppSubURL+ctx.Req.URL.RequestURI())
			}
			ctx.Redirect(setting.AppSubURL + "/user/login")
			return
		}

		if options.AdminRequired {
			if !ctx.Doer.IsAdmin {
				ctx.Error(http.StatusForbidden)
				return
			}
			ctx.Data["PageIsAdmin"] = true
		}
	}
}

// VerifyAuthWithOptionsAPI checks authentication according to options
func VerifyAuthWithOptionsAPI(options *VerifyOptions) func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		// Check prohibit login users.
		if ctx.IsSigned {
			if !ctx.Doer.IsActive && setting.Service.RegisterEmailConfirm {
				ctx.Data["Title"] = ctx.Tr("auth.active_your_account")
				ctx.JSON(http.StatusForbidden, map[string]string{
					"message": "This account is not activated.",
				})
				return
			}
			if !ctx.Doer.IsActive || ctx.Doer.ProhibitLogin {
				log.Info("Failed authentication attempt for %s from %s", ctx.Doer.Name, ctx.RemoteAddr())
				ctx.Data["Title"] = ctx.Tr("auth.prohibit_login")
				ctx.JSON(http.StatusForbidden, map[string]string{
					"message": "This account is prohibited from signing in, please contact your site administrator.",
				})
				return
			}

			if ctx.Doer.MustChangePassword {
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
			} else if !ctx.Doer.IsActive && setting.Service.RegisterEmailConfirm {
				ctx.Data["Title"] = ctx.Tr("auth.active_your_account")
				ctx.JSON(http.StatusForbidden, map[string]string{
					"message": "This account is not activated.",
				})
				return
			}
		}

		if options.AdminRequired {
			if !ctx.Doer.IsAdmin {
				ctx.JSON(http.StatusForbidden, map[string]string{
					"message": "You have no permission to request for this.",
				})
				return
			}
		}
	}
}
